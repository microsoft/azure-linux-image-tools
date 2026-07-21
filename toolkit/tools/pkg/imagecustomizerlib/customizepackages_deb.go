// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/cosiapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/sliceutils"
	"github.com/sirupsen/logrus"
	"github.com/spdx/tools-golang/spdx"
	"github.com/spdx/tools-golang/spdx/v2/common"
)

var (
	// Example: Setting up udev (255.4-1ubuntu8.16) ...
	aptLogInstall = regexp.MustCompile(`^Setting up (\S+) \((\S+)\) ...$`)

	// Example: Removing apt-utils (2.8.3) ...
	aptLogRemove = regexp.MustCompile(`^Removing (\S+) \((\S+)\) ...$`)
)

// managePackagesDeb orchestrates the complete DEB package management flow.
func managePackagesDeb(ctx context.Context, config *imagecustomizerapi.OS, imageChroot *safechroot.Chroot) error {
	if needPackageSources(config) {
		err := setupServicePrevention(imageChroot)
		if err != nil {
			return err
		}

		err = refreshDebPackageMetadata(ctx, imageChroot)
		if err != nil {
			return err
		}
	}

	err := removeDebPackages(ctx, config.Packages.Remove, imageChroot)
	if err != nil {
		return err
	}

	if config.Packages.UpdateExistingPackages {
		err = updateExistingDebPackages(ctx, imageChroot)
		if err != nil {
			return err
		}
	}

	err = installDebPackages(ctx, config.Packages.Install, imageChroot)
	if err != nil {
		return err
	}

	err = updateDebPackages(ctx, config.Packages.Update, imageChroot)
	if err != nil {
		return err
	}

	if needPackageCleanup(config) {
		err = cleanDebCache(ctx, imageChroot)
		if err != nil {
			return err
		}
	}

	if needPackageSources(config) {
		err = teardownServicePrevention(imageChroot)
		if err != nil {
			return err
		}
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
	err := shell.NewExecBuilder("dpkg-divert", "--local", "--rename", "--add", "/sbin/start-stop-daemon").
		ErrorStderrLines(1).
		Chroot(imageChroot.ChrootDir()).
		Execute()
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
	err := shell.NewExecBuilder("dpkg-divert", "--remove", "--rename", "/sbin/start-stop-daemon").
		ErrorStderrLines(1).
		Chroot(imageChroot.ChrootDir()).
		Execute()
	if err != nil {
		return fmt.Errorf("failed to restore start-stop-daemon via dpkg-divert:\n%w", err)
	}

	return nil
}

// refreshDebPackageMetadata runs apt-get update to refresh the package metadata.
func refreshDebPackageMetadata(ctx context.Context, imageChroot *safechroot.Chroot) error {
	logger.Log.Infof("Refreshing package metadata")

	_, span := startRefreshPackageMetadataSpan(ctx)
	defer span.End()

	args := []string{"update"}

	_, _, err := executeAptCommand(args, imageChroot)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrPackageRepoMetadataRefresh, err)
	}

	return nil
}

func executeAptCommand(args []string, imageChroot *safechroot.Chroot,
) ([]pkgNameAndVersion, []pkgNameAndVersion, error) {
	fullArgs := []string{"--yes",
		"--option", "Dpkg::Options::=--force-confdef",
		"--option", "Dpkg::Options::=--force-confold"}
	fullArgs = append(fullArgs, args...)

	env := append(shell.CurrentEnvironment(), getAptEnvironmentVariables()...)

	installedPackages := []pkgNameAndVersion(nil)
	removedPackages := []pkgNameAndVersion(nil)

	stdout := func(line string) {
		match := aptLogInstall.FindStringSubmatch(line)
		if match != nil {
			name := match[1]
			version := match[2]

			installedPackages = append(installedPackages, pkgNameAndVersion{
				Name:    name,
				Version: version,
			})
		}

		match = aptLogRemove.FindStringSubmatch(line)
		if match != nil {
			name := match[1]
			version := match[2]

			removedPackages = append(removedPackages, pkgNameAndVersion{
				Name:    name,
				Version: version,
			})
		}
	}

	err := shell.NewExecBuilder(packageManagerAPT, fullArgs...).
		EnvironmentVariables(env).
		LogLevel(logrus.DebugLevel, logrus.DebugLevel).
		StdoutCallback(stdout).
		ErrorStderrLines(1).
		Chroot(imageChroot.ChrootDir()).
		Execute()
	if err != nil {
		return nil, nil, err
	}

	return installedPackages, removedPackages, nil
}

// getAptEnvironmentVariables returns the environment variables required for non-interactive operations.
func getAptEnvironmentVariables() []string {
	return []string{
		"DEBIAN_FRONTEND=noninteractive",
		"DEBCONF_NONINTERACTIVE_SEEN=true",
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
	}
}

// installDebPackages runs apt-get install --no-install-recommends --no-install-suggests with the given package list.
func installDebPackages(ctx context.Context, packages []string, imageChroot *safechroot.Chroot) error {
	if len(packages) == 0 {
		return nil
	}

	logger.Log.Infof("Installing packages (%d): %v", len(packages), packages)

	_, span := startInstallPackagesSpan(ctx, packages)
	defer span.End()

	args := []string{"install", "--no-install-recommends", "--no-install-suggests"}
	args = append(args, packages...)

	_, _, err := executeAptCommand(args, imageChroot)
	if err != nil {
		return fmt.Errorf("%w (%v):\n%w", ErrPackageInstall, packages, err)
	}

	return nil
}

// removeDebPackages runs apt-get remove with the given package list, then
// apt-get autoremove to clean orphaned dependencies.
func removeDebPackages(ctx context.Context, packages []string, imageChroot *safechroot.Chroot) error {
	if len(packages) == 0 {
		return nil
	}

	logger.Log.Infof("Removing packages (%d): %v", len(packages), packages)

	_, span := startRemovePackagesSpan(ctx, packages)
	defer span.End()

	args := []string{"remove"}
	args = append(args, packages...)

	_, _, err := executeAptCommand(args, imageChroot)
	if err != nil {
		return fmt.Errorf("%w (%v):\n%w", ErrPackageRemove, packages, err)
	}

	args = []string{"autoremove"}

	_, _, err = executeAptCommand(args, imageChroot)
	if err != nil {
		return fmt.Errorf("%w (%v):\n%w", ErrPackageAutoRemove, packages, err)
	}

	return nil
}

func updateDebPackages(ctx context.Context, packages []string, imageChroot *safechroot.Chroot) error {
	if len(packages) == 0 {
		return nil
	}

	logger.Log.Infof("Updating packages (%d): %v", len(packages), packages)

	_, span := startUpdatePackagesSpan(ctx, packages)
	defer span.End()

	args := []string{"install", "--only-upgrade"}
	args = append(args, packages...)

	_, _, err := executeAptCommand(args, imageChroot)
	if err != nil {
		return fmt.Errorf("%w (%v):\n%w", ErrPackageUpdate, packages, err)
	}

	return nil
}

// updateExistingDebPackages runs apt-get upgrade.
func updateExistingDebPackages(ctx context.Context, imageChroot *safechroot.Chroot) error {
	logger.Log.Infof("Updating existing packages")

	_, span := startUpdateExistingPackagesSpan(ctx)
	defer span.End()

	args := []string{"upgrade"}

	_, _, err := executeAptCommand(args, imageChroot)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrPackagesUpdateInstalled, err)
	}

	return nil
}

// cleanDebCache cleans the DEB package cache via the package manager handler.
func cleanDebCache(ctx context.Context, imageChroot *safechroot.Chroot) error {
	logger.Log.Infof("Cleaning DEB cache")

	_, span := startCleanPackagesCacheSpan(ctx)
	defer span.End()

	args := []string{"clean"}

	_, _, err := executeAptCommand(args, imageChroot)
	if err != nil {
		return fmt.Errorf("failed to clean APT cache:\n%w", err)
	}

	// Remove APT lists.
	aptListsDir := filepath.Join(imageChroot.RootDir(), "var/lib/apt/lists")
	err = file.RemoveDirectoryContents(aptListsDir)
	if err != nil {
		return fmt.Errorf("failed to remove APT lists:\n%w", err)
	}

	// Truncate APT log files.
	aptLogDir := filepath.Join(imageChroot.RootDir(), "var/log/apt")
	logEntries, err := os.ReadDir(aptLogDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read log directory:\n%w", err)
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

// isPackageInstalledDeb checks if a package is installed using dpkg-query.
func isPackageInstalledDeb(imageChroot safechroot.ChrootInterface, packageName string) (bool, error) {
	err := shell.NewExecBuilder("dpkg-query", "-W", "-f='${Status}'", packageName).
		LogLevel(logrus.TraceLevel, logrus.DebugLevel).
		Chroot(imageChroot.ChrootDir()).
		Execute()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// The command ran and failed.
			return false, nil
		}

		// The command failed to start.
		return false, err
	}

	return true, nil
}

// debGetAllPackagesForCosi retrieves all installed packages from a DEB-based system for a COSI file.
func debGetAllPackagesForCosi(imageChroot safechroot.ChrootInterface) ([]cosiapi.OsPackage, error) {
	out, _, err := shell.NewExecBuilder("dpkg-query", "-W", "-f=${Package}\t${Version}\t${Architecture}\n").
		LogLevel(logrus.TraceLevel, logrus.DebugLevel).
		Chroot(imageChroot.ChrootDir()).
		ExecuteCaptureOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get dpkg output from chroot:\n%w", err)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	var packages []cosiapi.OsPackage
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
		packages = append(packages, cosiapi.OsPackage{
			Name:    parts[0],
			Version: parts[1],
			// dpkg doesn't have separate release
			Release: "",
			Arch:    parts[2],
		})
	}

	return packages, nil
}

// debGetOsManifestPackages
func debGetOsManifestPackages(imageChroot safechroot.ChrootInterface) (osManifestPackages, error) {
	out, _, err := shell.NewExecBuilder("dpkg-query", "-W", "-f=${Package}\t${Version}\t${Architecture}\n").
		LogLevel(logrus.TraceLevel, logrus.DebugLevel).
		Chroot(imageChroot.ChrootDir()).
		ExecuteCaptureOutput()
	if err != nil {
		return osManifestPackages{}, fmt.Errorf("failed to get dpkg output from chroot:\n%w", err)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	var packages []*spdx.Package
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			return osManifestPackages{}, fmt.Errorf("malformed dpkg line encountered while parsing installed packages: %q", line)
		}

		name := parts[0]
		version := parts[1]
		arch := parts[2]

		packageIdBytes := sha256.Sum256(fmt.Appendf(nil, "%s-%s.%s", name, version, arch))
		packageId := hex.EncodeToString(packageIdBytes[:])

		packages = append(packages, &spdx.Package{
			PackageName:             name,
			PackageSPDXIdentifier:   spdx.ElementID(fmt.Sprintf("Package-%s", packageId)),
			PackageVersion:          version,
			PackageDownloadLocation: "NOASSERTION",
			FilesAnalyzed:           false,
			PackageLicenseConcluded: "NOASSERTION",
			PackageLicenseDeclared:  "NOASSERTION",
			PackageCopyrightText:    "NOASSERTION",
			PackageSupplier: &common.Supplier{
				Supplier: "NOASSERTION",
			},
		})
	}

	manifest := osManifestPackages{
		Packages:      packages,
		Relationships: nil,
	}

	return manifest, nil
}

func debRemovePackageManagerTools(imageChroot *safechroot.Chroot, packageManagementPackages []string,
) (osManifestPackages, error) {
	// Once the package manager tools have been removed, it will no longer be possible to collect the package list.
	// So, collect it now.
	manifest, err := debGetOsManifestPackages(imageChroot)
	if err != nil {
		return osManifestPackages{}, fmt.Errorf("%w:\n%w", ErrCollectManifestPackages, err)
	}

	removedPackages, err := debEnsurePackagesRemoved(imageChroot, packageManagementPackages, true /*removeEssentialPackages*/)
	if err != nil {
		return osManifestPackages{}, err
	}

	removedPackagesSet := sliceutils.SliceToSet(removedPackages)

	manifest.Filter(func(packageInfo *spdx.Package) bool {
		_, packageRemoved := removedPackagesSet[pkgNameAndVersion{packageInfo.PackageName, packageInfo.PackageVersion}]
		return !packageRemoved
	})

	return manifest, nil
}

func debEnsurePackagesRemoved(imageChroot *safechroot.Chroot, packages []string, removeEssentialPackages bool,
) ([]pkgNameAndVersion, error) {
	packagesToRemove := []string(nil)
	for _, packageName := range packages {
		installed, err := isPackageInstalledDeb(imageChroot, packageName)
		if err != nil {
			return nil, err
		}

		if installed {
			packagesToRemove = append(packagesToRemove, packageName)
		}
	}

	if len(packagesToRemove) <= 0 {
		// Nothing to do.
		return nil, nil
	}

	args := []string{"remove", "--auto-remove"}
	if removeEssentialPackages {
		args = append(args, "--allow-remove-essential")
	}
	args = append(args, "--")
	args = append(args, packagesToRemove...)

	_, removedPackages, err := executeAptCommand(args, imageChroot)
	if err != nil {
		return nil, fmt.Errorf("%w (%v):\n%w", ErrPackageRemove, packagesToRemove, err)
	}

	return removedPackages, nil
}
