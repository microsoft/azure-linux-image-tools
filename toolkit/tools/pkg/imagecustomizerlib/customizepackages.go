// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// packageManager defines the interface for distro-specific package management operations.
type packageManager interface {
	// getBinaryName returns the name of the package manager binary (e.g., "tdnf", "dnf", "apt-get")
	getBinaryName() string

	// getDownloadRegex returns the regex pattern for parsing download progress lines
	getDownloadRegex() *regexp.Regexp

	// getTransactionErrorRegex returns the regex pattern for detecting transaction errors
	getTransactionErrorRegex() *regexp.Regexp

	// getSummaryLines returns the lines that indicate the start of operation summaries
	getSummaryLines() []string

	// getOpLines returns the prefixes that indicate operation lines
	getOpLines() []string

	// appendArgsForToolsChroot modifies package manager arguments for tools chroot operations
	appendArgsForToolsChroot(args []string) []string

	// Package management operations - return command arguments for each operation
	getInstallCommand(packages []string, extraOptions map[string]string) []string
	getRemoveCommand(packages []string, extraOptions map[string]string) []string
	getUpdateAllCommand(extraOptions map[string]string) []string
	getUpdateCommand(packages []string, extraOptions map[string]string) []string
	getCleanCommand() []string
	getRefreshMetadataCommand(extraOptions map[string]string) []string

	// Package manager capabilities
	requiresMetadataRefresh() bool
	supportsCacheOnly() bool
	supportsRepoConfiguration() bool
}

// Factory function to create package managers based on distro name
func newPackageManager(distroName string) packageManager {
	switch distroName {
	case "azurelinux":
		return newTdnfPackageManager()
	case "fedora":
		return newDnfPackageManager()
	default:
		// Default to Azure Linux for backward compatibility
		return newTdnfPackageManager()
	}
}

// removePackagesHelper provides a common implementation for removing packages
func removePackagesHelper(ctx context.Context, packages []string, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, pm packageManager, repoDir string) error {
	logger.Log.Infof("Removing packages: %v", packages)

	if len(packages) <= 0 {
		return nil
	}

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "remove_packages")
	pkgJson, _ := json.Marshal(packages)
	span.SetAttributes(
		attribute.Int("remove_packages_count", len(packages)),
		attribute.String("remove_packages", string(pkgJson)),
	)
	defer span.End()

	// Create extra options map for package manager specific configurations
	extraOptions := make(map[string]string)
	if pm.supportsRepoConfiguration() && repoDir != "" {
		extraOptions["reposdir"] = repoDir
	}

	removeArgs := pm.getRemoveCommand(packages, extraOptions)

	err := callPackageManager(removeArgs, imageChroot, toolsChroot, pm)
	if err != nil {
		return fmt.Errorf("failed to remove packages (%v):\n%w", packages, err)
	}

	return nil
}

// updateAllPackagesHelper provides a common implementation for updating all packages
func updateAllPackagesHelper(ctx context.Context, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, pm packageManager, repoDir string) error {
	logger.Log.Infof("Updating base image packages")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "update_base_packages")
	defer span.End()

	// Create extra options map for package manager specific configurations
	extraOptions := make(map[string]string)
	if pm.supportsCacheOnly() {
		extraOptions["cacheonly"] = "true"
	}
	if pm.supportsRepoConfiguration() && repoDir != "" {
		extraOptions["reposdir"] = repoDir
	}

	updateArgs := pm.getUpdateAllCommand(extraOptions)

	err := callPackageManager(updateArgs, imageChroot, toolsChroot, pm)
	if err != nil {
		return fmt.Errorf("failed to update packages:\n%w", err)
	}

	return nil
}

// installOrUpdatePackagesHelper provides a common implementation for installing or updating packages
func installOrUpdatePackagesHelper(ctx context.Context, action string, packages []string, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, pm packageManager, repoDir string) error {
	if len(packages) <= 0 {
		return nil
	}

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, action+"_packages")
	pkgJson, _ := json.Marshal(packages)
	span.SetAttributes(
		attribute.Int(action+"_packages_count", len(packages)),
		attribute.String("packages", string(pkgJson)),
	)
	defer span.End()

	// Create extra options map for package manager specific configurations
	extraOptions := make(map[string]string)
	if pm.supportsCacheOnly() {
		extraOptions["cacheonly"] = "true"
	}
	if pm.supportsRepoConfiguration() && repoDir != "" {
		extraOptions["reposdir"] = repoDir
	}

	var installArgs []string
	if action == "install" {
		installArgs = pm.getInstallCommand(packages, extraOptions)
	} else if action == "update" {
		installArgs = pm.getUpdateCommand(packages, extraOptions)
	} else {
		return fmt.Errorf("unsupported action: %s", action)
	}

	err := callPackageManager(installArgs, imageChroot, toolsChroot, pm)
	if err != nil {
		return fmt.Errorf("failed to %s packages (%v):\n%w", action, packages, err)
	}

	return nil
}

// cleanCacheHelper provides a common implementation for cleaning package manager cache
func cleanCacheHelper(imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, pm packageManager) error {
	logger.Log.Infof("Cleaning up package manager cache")

	cleanArgs := pm.getCleanCommand()

	pmChroot := imageChroot
	if toolsChroot != nil {
		cleanArgs = pm.appendArgsForToolsChroot(cleanArgs)
		pmChroot = toolsChroot
	}

	return pmChroot.UnsafeRun(func() error {
		err := shell.NewExecBuilder(pm.getBinaryName(), cleanArgs...).
			LogLevel(logrus.TraceLevel, logrus.DebugLevel).
			ErrorStderrLines(1).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to clean %s cache:\n%w", pm.getBinaryName(), err)
		}
		return nil
	})
}

// refreshMetadataHelper provides a common implementation for refreshing package metadata
func refreshMetadataHelper(ctx context.Context, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, pm packageManager, repoDir string) error {
	if !pm.requiresMetadataRefresh() {
		logger.Log.Debug("Package manager does not require metadata refresh, skipping")
		return nil
	}

	logger.Log.Infof("Refreshing package metadata")

	// Create extra options map for package manager specific configurations
	extraOptions := make(map[string]string)
	if pm.supportsRepoConfiguration() && repoDir != "" {
		extraOptions["reposdir"] = repoDir
	}

	refreshArgs := pm.getRefreshMetadataCommand(extraOptions)

	pmChroot := imageChroot
	if toolsChroot != nil {
		refreshArgs = pm.appendArgsForToolsChroot(refreshArgs)
		pmChroot = toolsChroot
	}

	return pmChroot.UnsafeRun(func() error {
		err := shell.NewExecBuilder(pm.getBinaryName(), refreshArgs...).
			LogLevel(logrus.TraceLevel, logrus.DebugLevel).
			ErrorStderrLines(1).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to refresh package metadata:\n%w", err)
		}
		return nil
	})
}

// callPackageManager executes a package manager command with proper logging and error handling
func callPackageManager(pmArgs []string, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, pm packageManager) error {
	pmChroot := imageChroot
	if toolsChroot != nil {
		pmArgs = pm.appendArgsForToolsChroot(pmArgs)
		pmChroot = toolsChroot
	}
	if _, err := os.Stat(filepath.Join(pmChroot.RootDir(), customTdnfConfRelPath)); err == nil {
		pmArgs = append([]string{"--config", "/" + customTdnfConfRelPath}, pmArgs...)
	}

	lastDownloadPackageSeen := ""
	inSummary := false
	seenTransactionErrorMessage := false
	stdoutCallback := func(line string) {
		if !seenTransactionErrorMessage {
			seenTransactionErrorMessage = pm.getTransactionErrorRegex().MatchString(line)
		}

		switch {
		case seenTransactionErrorMessage:
			logger.Log.Warn(line)

		case inSummary && line == "":
			inSummary = false
			logger.Log.Trace(line)

		case inSummary:
			logger.Log.Debug(line)

		case slices.Contains(pm.getSummaryLines(), line):
			inSummary = true
			logger.Log.Debug(line)

		case slices.ContainsFunc(pm.getOpLines(), func(opPrefix string) bool { return strings.HasPrefix(line, opPrefix) }):
			logger.Log.Debug(line)

		default:
			match := pm.getDownloadRegex().FindStringSubmatch(line)
			if match != nil {
				packageName := match[1]
				if packageName != lastDownloadPackageSeen {
					lastDownloadPackageSeen = packageName
					logger.Log.Debug(line)
					break
				}
			}

			logger.Log.Trace(line)
		}
	}

	return pmChroot.UnsafeRun(func() error {
		return shell.NewExecBuilder(pm.getBinaryName(), pmArgs...).
			StdoutCallback(stdoutCallback).
			LogLevel(shell.LogDisabledLevel, logrus.DebugLevel).
			ErrorStderrLines(1).
			Execute()
	})
}

func addRemoveAndUpdatePackages(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.OS,
	imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	rpmsSources []string, useBaseImageRpmRepos bool, distroName string, snapshotTime string,
) error {
	var err error

	ctx, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "configure_packages")
	defer span.End()

	if snapshotTime == "" {
		snapshotTime = string(config.Packages.SnapshotTime)
	}

	// Create package manager instance based on distro
	packageManager := newPackageManager(distroName)
	packageManagerChroot := imageChroot
	if toolsChroot != nil {
		packageManagerChroot = toolsChroot
	}

	err = createTempTdnfConfigWithSnapshot(packageManagerChroot, imagecustomizerapi.PackageSnapshotTime(snapshotTime))
	if err != nil {
		return err
	}
	defer func() {
		if cleanupErr := cleanupSnapshotTimeConfig(packageManagerChroot); cleanupErr != nil && err == nil {
			err = cleanupErr
		}
	}()

	// Note: The 'validatePackageLists' function read the PackageLists files and merged them into the inline package lists.
	needRpmsSources := len(config.Packages.Install) > 0 || len(config.Packages.Update) > 0 ||
		config.Packages.UpdateExistingPackages

	var mounts *rpmSourcesMounts
	if needRpmsSources {
		// Mount RPM sources.
		mounts, err = mountRpmSources(ctx, buildDir, packageManagerChroot, rpmsSources, useBaseImageRpmRepos)
		if err != nil {
			return err
		}
		defer mounts.close()

		// Refresh metadata.
		err = refreshMetadataHelper(ctx, imageChroot, toolsChroot, packageManager, rpmsMountParentDirInChroot)
		if err != nil {
			return err
		}
	}

	err = removePackagesHelper(ctx, config.Packages.Remove, imageChroot, toolsChroot, packageManager, rpmsMountParentDirInChroot)
	if err != nil {
		return err
	}

	if config.Packages.UpdateExistingPackages {
		err = updateAllPackagesHelper(ctx, imageChroot, toolsChroot, packageManager, rpmsMountParentDirInChroot)
		if err != nil {
			return err
		}
	}

	logger.Log.Infof("Installing packages: %v", config.Packages.Install)
	err = installOrUpdatePackagesHelper(ctx, "install", config.Packages.Install, imageChroot, toolsChroot, packageManager, rpmsMountParentDirInChroot)
	if err != nil {
		return err
	}

	logger.Log.Infof("Updating packages: %v", config.Packages.Update)
	err = installOrUpdatePackagesHelper(ctx, "update", config.Packages.Update, imageChroot, toolsChroot, packageManager, rpmsMountParentDirInChroot)
	if err != nil {
		return err
	}

	// Unmount RPM sources.
	if mounts != nil {
		err = mounts.close()
		if err != nil {
			return err
		}
	}

	if needRpmsSources {
		err = cleanCacheHelper(imageChroot, toolsChroot, packageManager)
		if err != nil {
			return err
		}
	}

	return nil
}

func collectPackagesList(baseConfigPath string, packageLists []string, packages []string) ([]string, error) {
	var err error

	// Read in the packages from the package list files.
	var allPackages []string
	for _, packageListRelativePath := range packageLists {
		packageListFilePath := file.GetAbsPathWithBase(baseConfigPath, packageListRelativePath)

		var packageList imagecustomizerapi.PackageList
		err = imagecustomizerapi.UnmarshalAndValidateYamlFile(packageListFilePath, &packageList)
		if err != nil {
			return nil, fmt.Errorf("failed to read package list file (%s):\n%w", packageListFilePath, err)
		}

		allPackages = append(allPackages, packageList.Packages...)
	}

	allPackages = append(allPackages, packages...)
	return allPackages, nil
}

// TODO: update this function when adding support for fedora for image customizer
func isPackageInstalled(imageChroot safechroot.ChrootInterface, packageName string) bool {
	err := imageChroot.UnsafeRun(func() error {
		_, _, err := shell.Execute("tdnf", "info", packageName, "--repo", "@system")
		return err
	})
	return err == nil
}
