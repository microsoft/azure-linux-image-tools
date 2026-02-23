// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// aptEnvironmentVariables returns the environment variables required for non-interactive apt-get operations.
func aptEnvironmentVariables() []string {
	return []string{
		"DEBIAN_FRONTEND=noninteractive",
		"DEBCONF_NONINTERACTIVE_SEEN=true",
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
	}
}

// dpkgPreFlight removes stale dpkg lock files and runs dpkg --configure -a unconditionally.
// All failures are warning-only and do not prevent the build from continuing.
func dpkgPreFlight(imageChroot *safechroot.Chroot) {
	lockFiles := []string{
		"/var/lib/dpkg/lock",
		"/var/lib/dpkg/lock-frontend",
		"/var/lib/apt/lists/lock",
		"/var/cache/apt/archives/lock",
	}

	for _, lock := range lockFiles {
		fullPath := filepath.Join(imageChroot.RootDir(), lock)
		if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
			logger.Log.Warnf("Failed to remove dpkg lock file %s: %v", lock, err)
		}
	}

	env := append(shell.CurrentEnvironment(), aptEnvironmentVariables()...)

	err := imageChroot.UnsafeRun(func() error {
		return shell.NewExecBuilder("dpkg", "--configure", "-a").
			EnvironmentVariables(env).
			LogLevel(shell.LogDisabledLevel, logrus.DebugLevel).
			Execute()
	})
	if err != nil {
		logger.Log.Warnf("dpkg pre-flight --configure -a failed: %v", err)
	}
}

// setupServicePrevention creates policy-rc.d and diverts start-stop-daemon to prevent
// services from auto-starting during package installation inside the chroot.
func setupServicePrevention(imageChroot *safechroot.Chroot) error {
	// Create /usr/sbin/policy-rc.d to prevent invoke-rc.d from starting services.
	policyRcPath := filepath.Join(imageChroot.RootDir(), "usr/sbin/policy-rc.d")
	if _, err := os.Stat(policyRcPath); os.IsNotExist(err) {
		err = os.WriteFile(policyRcPath, []byte("#!/bin/sh\nexit 101\n"), 0755)
		if err != nil {
			return fmt.Errorf("failed to create policy-rc.d:\n%w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to check policy-rc.d:\n%w", err)
	} else {
		logger.Log.Infof("policy-rc.d already exists, skipping creation")
	}

	// Divert start-stop-daemon so that dpkg post-install hooks cannot start daemons.
	err := imageChroot.UnsafeRun(func() error {
		return shell.NewExecBuilder("dpkg-divert", "--local", "--rename", "--add", "/sbin/start-stop-daemon").
			ErrorStderrLines(1).
			Execute()
	})
	if err != nil {
		return fmt.Errorf("failed to divert start-stop-daemon:\n%w", err)
	}

	// Create a no-op replacement for start-stop-daemon.
	startStopDaemonPath := filepath.Join(imageChroot.RootDir(), "sbin/start-stop-daemon")
	err = os.WriteFile(startStopDaemonPath, []byte("#!/bin/sh\nexit 0\n"), 0755)
	if err != nil {
		return fmt.Errorf("failed to create no-op start-stop-daemon:\n%w", err)
	}

	return nil
}

// teardownServicePrevention removes the policy-rc.d file and restores the original
// start-stop-daemon. Returns an error if dpkg-divert removal fails, as the user decided
// that cleanup failure should fail the build.
func teardownServicePrevention(imageChroot *safechroot.Chroot) error {
	// Remove policy-rc.d.
	policyRcPath := filepath.Join(imageChroot.RootDir(), "usr/sbin/policy-rc.d")
	if err := os.Remove(policyRcPath); err != nil && !os.IsNotExist(err) {
		logger.Log.Warnf("Failed to remove policy-rc.d: %v", err)
	}

	// Remove the no-op start-stop-daemon before restoring the original via dpkg-divert.
	startStopDaemonPath := filepath.Join(imageChroot.RootDir(), "sbin/start-stop-daemon")
	if err := os.Remove(startStopDaemonPath); err != nil && !os.IsNotExist(err) {
		logger.Log.Warnf("Failed to remove no-op start-stop-daemon: %v", err)
	}

	// Restore the original start-stop-daemon via dpkg-divert.
	err := imageChroot.UnsafeRun(func() error {
		return shell.NewExecBuilder("dpkg-divert", "--remove", "--rename", "/sbin/start-stop-daemon").
			ErrorStderrLines(1).
			Execute()
	})
	if err != nil {
		return fmt.Errorf("failed to restore start-stop-daemon via dpkg-divert:\n%w", err)
	}

	return nil
}

// refreshAptPackageMetadata runs apt-get update to refresh the package metadata
// from the base image repositories.
func refreshAptPackageMetadata(ctx context.Context, imageChroot *safechroot.Chroot) error {
	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "refresh_metadata")
	defer span.End()

	logger.Log.Infof("Refreshing package metadata")

	env := append(shell.CurrentEnvironment(), aptEnvironmentVariables()...)

	err := imageChroot.UnsafeRun(func() error {
		return shell.NewExecBuilder("apt-get", "update", "-y").
			EnvironmentVariables(env).
			LogLevel(shell.LogDisabledLevel, logrus.DebugLevel).
			ErrorStderrLines(1).
			Execute()
	})
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrPackageRepoMetadataRefresh, err)
	}

	return nil
}

// installAptPackages runs apt-get install with --no-install-recommends and --no-install-suggests
// for the given list of packages.
func installAptPackages(ctx context.Context, packages []string, imageChroot *safechroot.Chroot) error {
	if len(packages) == 0 {
		return nil
	}

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "install_packages")
	span.SetAttributes(
		attribute.Int("install_packages_count", len(packages)),
		attribute.StringSlice("packages", packages),
	)
	defer span.End()

	logger.Log.Infof("Installing packages (%d): %v (using --no-install-recommends)", len(packages), packages)

	args := []string{
		"install", "-y",
		"--no-install-recommends", "--no-install-suggests",
		"-o", "Dpkg::Options::=--force-confdef",
		"-o", "Dpkg::Options::=--force-confold",
	}
	args = append(args, packages...)

	env := append(shell.CurrentEnvironment(), aptEnvironmentVariables()...)

	err := imageChroot.UnsafeRun(func() error {
		return shell.NewExecBuilder("apt-get", args...).
			EnvironmentVariables(env).
			LogLevel(shell.LogDisabledLevel, logrus.DebugLevel).
			ErrorStderrLines(1).
			Execute()
	})
	if err != nil {
		return fmt.Errorf("%w (%v):\n%w", ErrPackageInstall, packages, err)
	}

	return nil
}

// cleanAptCache runs apt-get clean, removes apt lists, and truncates log files.
// All failures are warning-only and do not prevent the build from continuing.
func cleanAptCache(ctx context.Context, imageChroot *safechroot.Chroot) {
	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "clean_cache")
	defer span.End()

	logger.Log.Infof("Cleaning up APT cache")

	env := append(shell.CurrentEnvironment(), aptEnvironmentVariables()...)

	// apt-get clean
	err := imageChroot.UnsafeRun(func() error {
		return shell.NewExecBuilder("apt-get", "clean").
			EnvironmentVariables(env).
			LogLevel(shell.LogDisabledLevel, logrus.DebugLevel).
			ErrorStderrLines(1).
			Execute()
	})
	if err != nil {
		logger.Log.Warnf("Failed to clean APT cache: %v", err)
	}

	// rm -rf /var/lib/apt/lists/* (must use sh -c for glob expansion)
	err = imageChroot.UnsafeRun(func() error {
		return shell.NewExecBuilder("/bin/sh", "-c", "rm -rf /var/lib/apt/lists/*").
			Execute()
	})
	if err != nil {
		logger.Log.Warnf("Failed to remove APT lists: %v", err)
	}

	// Truncate apt and dpkg log files
	err = imageChroot.UnsafeRun(func() error {
		return shell.NewExecBuilder("/bin/sh", "-c", "truncate -s 0 /var/log/apt/*.log /var/log/dpkg.log 2>/dev/null || true").
			Execute()
	})
	if err != nil {
		logger.Log.Warnf("Failed to truncate APT/dpkg log files: %v", err)
	}
}

// managePackagesApt orchestrates the complete APT package management flow:
// pre-flight → service prevention → update → install → clean → teardown.
func managePackagesApt(ctx context.Context, baseConfigPath string, config *imagecustomizerapi.OS,
	imageChroot *safechroot.Chroot,
) (err error) {
	if len(config.Packages.Install) == 0 {
		return nil
	}

	// Pre-flight: clean dpkg locks and run dpkg --configure -a (warning only).
	dpkgPreFlight(imageChroot)

	// Setup service prevention (policy-rc.d + start-stop-daemon diversion).
	err = setupServicePrevention(imageChroot)
	if err != nil {
		return err
	}
	defer func() {
		cleanupErr := teardownServicePrevention(imageChroot)
		if cleanupErr != nil && err == nil {
			err = cleanupErr
		}
	}()

	// Refresh package metadata (fatal on failure).
	err = refreshAptPackageMetadata(ctx, imageChroot)
	if err != nil {
		return err
	}

	// Install packages (fatal on failure, skip clean).
	err = installAptPackages(ctx, config.Packages.Install, imageChroot)
	if err != nil {
		return err
	}

	// Clean APT cache (warning only, non-fatal).
	cleanAptCache(ctx, imageChroot)

	return nil
}
