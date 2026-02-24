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
// start-stop-daemon. Returns an error on any failure, since cleanup failures should
// fail the build.
func teardownServicePrevention(imageChroot *safechroot.Chroot) error {
	// Remove policy-rc.d.
	policyRcPath := filepath.Join(imageChroot.RootDir(), "usr/sbin/policy-rc.d")
	if err := os.Remove(policyRcPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove policy-rc.d:\n%w", err)
	}

	// Remove the no-op start-stop-daemon before restoring the original via dpkg-divert.
	startStopDaemonPath := filepath.Join(imageChroot.RootDir(), "sbin/start-stop-daemon")
	if err := os.Remove(startStopDaemonPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove no-op start-stop-daemon:\n%w", err)
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
	_, span := startPackagesSpan(ctx, packageSpanNameRefreshMetadata)
	defer span.End()

	logger.Log.Infof("Refreshing package metadata")

	env := append(shell.CurrentEnvironment(), aptEnvironmentVariables()...)

	err := imageChroot.UnsafeRun(func() error {
		return shell.NewExecBuilder("apt-get", "update", "-y").
			EnvironmentVariables(env).
			LogLevel(logrus.DebugLevel, logrus.DebugLevel).
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

	_, span := startPackageListSpan(ctx, packageActionInstall, packages)
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
			LogLevel(logrus.DebugLevel, logrus.DebugLevel).
			ErrorStderrLines(1).
			Execute()
	})
	if err != nil {
		return fmt.Errorf("%w (%v):\n%w", ErrPackageInstall, packages, err)
	}

	return nil
}

// cleanAptCache runs apt-get clean, removes apt lists, and truncates log files.
func cleanAptCache(ctx context.Context, imageChroot *safechroot.Chroot) error {
	_, span := startPackagesSpan(ctx, packageSpanNameCleanCache)
	defer span.End()

	logger.Log.Infof("Cleaning up APT cache")

	env := append(shell.CurrentEnvironment(), aptEnvironmentVariables()...)

	// apt-get clean
	err := imageChroot.UnsafeRun(func() error {
		return shell.NewExecBuilder("apt-get", "clean").
			EnvironmentVariables(env).
			LogLevel(logrus.DebugLevel, logrus.DebugLevel).
			ErrorStderrLines(1).
			Execute()
	})
	if err != nil {
		return fmt.Errorf("failed to clean APT cache:\n%w", err)
	}

	// Remove APT lists.
	aptListsDir := filepath.Join(imageChroot.RootDir(), "var/lib/apt/lists")
	err = removeDirectoryContents(aptListsDir)
	if err != nil {
		return fmt.Errorf("failed to remove APT lists:\n%w", err)
	}

	// Truncate APT log files.
	aptLogDir := filepath.Join(imageChroot.RootDir(), "var/log/apt")
	logEntries, err := os.ReadDir(aptLogDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read APT log directory:\n%w", err)
	}

	for _, entry := range logEntries {
		if entry.IsDir() {
			continue
		}

		if filepath.Ext(entry.Name()) != ".log" {
			continue
		}

		fullPath := filepath.Join(aptLogDir, entry.Name())
		err = os.Truncate(fullPath, 0)
		if err != nil {
			return fmt.Errorf("failed to truncate log file (%s):\n%w", entry.Name(), err)
		}
	}

	// Truncate dpkg log file.
	dpkgLogPath := filepath.Join(imageChroot.RootDir(), "var/log/dpkg.log")
	err = os.Truncate(dpkgLogPath, 0)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to truncate log file (var/log/dpkg.log):\n%w", err)
	}

	return nil
}

// removeDirectoryContents removes all files and subdirectories inside a directory,
// but preserves the directory itself.
func removeDirectoryContents(dirPath string) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read directory (%s):\n%w", dirPath, err)
	}

	for _, entry := range entries {
		entryPath := filepath.Join(dirPath, entry.Name())
		err = os.RemoveAll(entryPath)
		if err != nil {
			return fmt.Errorf("failed to remove (%s):\n%w", entryPath, err)
		}
	}

	return nil
}

// managePackagesApt orchestrates the complete APT package management flow:
// service prevention → update → install → clean → teardown.
func managePackagesApt(ctx context.Context, baseConfigPath string, config *imagecustomizerapi.OS,
	imageChroot *safechroot.Chroot,
) error {
	if len(config.Packages.Install) == 0 {
		return nil
	}

	// Setup service prevention (policy-rc.d + start-stop-daemon diversion).
	err := setupServicePrevention(imageChroot)
	if err != nil {
		return err
	}

	// Refresh package metadata (fatal on failure).
	err = refreshAptPackageMetadata(ctx, imageChroot)
	if err != nil {
		return err
	}

	// Install packages (fatal on failure).
	err = installAptPackages(ctx, config.Packages.Install, imageChroot)
	if err != nil {
		return err
	}

	// Clean APT cache.
	err = cleanAptCache(ctx, imageChroot)
	if err != nil {
		return err
	}

	// Teardown service prevention (restore start-stop-daemon and remove policy-rc.d).
	err = teardownServicePrevention(imageChroot)
	if err != nil {
		return err
	}

	return nil
}
