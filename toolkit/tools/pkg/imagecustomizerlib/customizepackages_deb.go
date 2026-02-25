// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/sirupsen/logrus"
)

// managePackagesDeb orchestrates the complete DEB package management flow:
// service prevention → update → install → clean → teardown.
func managePackagesDeb(ctx context.Context, config *imagecustomizerapi.OS,
	imageChroot *safechroot.Chroot, pmHandler debPackageManagerHandler,
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
	err = refreshDebPackageMetadata(ctx, imageChroot, pmHandler)
	if err != nil {
		return err
	}

	// Install packages (fatal on failure).
	err = installDebPackages(ctx, config.Packages.Install, imageChroot, pmHandler)
	if err != nil {
		return err
	}

	// Clean DEB cache.
	err = cleanDebCache(ctx, imageChroot, pmHandler)
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
		return fmt.Errorf("policy-rc.d already exists")
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
	if _, err := os.Stat(startStopDaemonPath); err == nil {
		return fmt.Errorf("start-stop-daemon already exists after divert")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check start-stop-daemon:\n%w", err)
	}

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

// refreshDebPackageMetadata runs apt-get update to refresh the package metadata.
func refreshDebPackageMetadata(ctx context.Context, imageChroot *safechroot.Chroot,
	pmHandler debPackageManagerHandler,
) error {
	logger.Log.Infof("Refreshing package metadata")

	_, span := startRefreshMetadataSpan(ctx)
	defer span.End()

	env := append(shell.CurrentEnvironment(), pmHandler.getEnvironmentVariables()...)

	err := imageChroot.UnsafeRun(func() error {
		return shell.NewExecBuilder(pmHandler.getPackageManagerBinary(), "update", "-y").
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

// installDebPackages runs apt-get install with the given list of packages.
func installDebPackages(ctx context.Context, packages []string, imageChroot *safechroot.Chroot,
	pmHandler debPackageManagerHandler,
) error {
	if len(packages) == 0 {
		return nil
	}

	logger.Log.Infof("Installing packages (%d): %v", len(packages), packages)

	_, span := startInstallPackagesSpan(ctx, packages)
	defer span.End()

	args := []string{
		"install", "-y",
		"--no-install-recommends", "--no-install-suggests",
		"-o", "Dpkg::Options::=--force-confdef",
		"-o", "Dpkg::Options::=--force-confold",
	}
	args = append(args, packages...)

	env := append(shell.CurrentEnvironment(), pmHandler.getEnvironmentVariables()...)

	err := imageChroot.UnsafeRun(func() error {
		return shell.NewExecBuilder(pmHandler.getPackageManagerBinary(), args...).
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

// cleanDebCache cleans the DEB package cache via the package manager handler.
func cleanDebCache(ctx context.Context, imageChroot *safechroot.Chroot,
	pmHandler debPackageManagerHandler,
) error {
	logger.Log.Infof("Cleaning DEB cache")

	_, span := startCleanCacheSpan(ctx)
	defer span.End()

	err := pmHandler.cleanPackageCache(imageChroot)
	if err != nil {
		return err
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

// isPackageInstalledDeb checks if a package is installed using dpkg-query.
func isPackageInstalledDeb(imageChroot safechroot.ChrootInterface, packageName string) bool {
	err := imageChroot.UnsafeRun(func() error {
		_, _, err := shell.Execute("dpkg-query", "-W", "-f='${Status}'", packageName)
		return err
	})
	if err != nil {
		return false
	}
	return true
}

// getAllPackagesFromChrootDeb retrieves all installed packages from a DEB-based system.
func getAllPackagesFromChrootDeb(imageChroot safechroot.ChrootInterface) ([]OsPackage, error) {
	var out string
	err := imageChroot.UnsafeRun(func() error {
		var err error
		// Query format: package:arch version architecture
		out, _, err = shell.Execute(
			"dpkg-query", "-W", "-f=${Package}\t${Version}\t${Architecture}\n",
		)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get dpkg output from chroot:\n%w", err)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	var packages []OsPackage
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			return nil, fmt.Errorf("malformed dpkg line encountered while parsing installed packages for COSI: %q", line)
		}

		// For dpkg, it does not have a separate release field.
		// Version contains epoch:version-release, use the whole thing as version.
		packages = append(packages, OsPackage{
			Name:    parts[0],
			Version: parts[1],
			// dpkg doesn't have separate release
			Release: "",
			Arch:    parts[2],
		})
	}

	return packages, nil
}
