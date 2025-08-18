// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// executePackageManagerCommand runs a package manager command with proper chroot handling
func executePackageManagerCommand(args []string, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, isToolsCommand bool, distroConfig DistroConfig) error {
	pmChroot := imageChroot
	if toolsChroot != nil && isToolsCommand {
		pmChroot = toolsChroot
	}

	// Add package manager specific config file if available
	if distroConfig.GetConfigFile() != "" && distroConfig.GetPackageType() == "rpm" {
		if _, err := os.Stat(filepath.Join(pmChroot.RootDir(), distroConfig.GetConfigFile())); err == nil {
			args = append([]string{"--config", "/" + distroConfig.GetConfigFile()}, args...)
		}
	}

	return pmChroot.UnsafeRun(func() error {
		return shell.NewExecBuilder(distroConfig.GetPackageManagerBinary(), args...).
			LogLevel(logrus.TraceLevel, logrus.DebugLevel).
			ErrorStderrLines(1).
			Execute()
	})
}

func installOrUpdatePackages(ctx context.Context, operation string, packages []string, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, distroConfig DistroConfig, repoDir string) error {
	if len(packages) == 0 {
		return nil
	}

	// Build command arguments directly based on distro and operation
	var args []string
	useToolsChroot := toolsChroot != nil

	if operation == "install" {
		args = buildInstallArgs(distroConfig, packages, repoDir, useToolsChroot)
	} else {
		args = buildUpdateArgs(distroConfig, packages, repoDir, useToolsChroot)
	}

	err := executePackageManagerCommand(args, imageChroot, toolsChroot, useToolsChroot, distroConfig)
	if err != nil {
		return fmt.Errorf("failed to %s packages (%v):\n%w", operation, packages, err)
	}
	return nil
}

// updateAllPackages updates all packages using the appropriate package manager
func updateAllPackages(ctx context.Context, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, distroConfig DistroConfig, repoDir string) error {
	logger.Log.Infof("Updating base image packages")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "update_base_packages")
	defer span.End()

	useToolsChroot := toolsChroot != nil
	args := buildUpdateAllArgs(distroConfig, repoDir, useToolsChroot)
	err := executePackageManagerCommand(args, imageChroot, toolsChroot, useToolsChroot, distroConfig)
	if err != nil {
		return fmt.Errorf("failed to update packages:\n%w", err)
	}
	return nil
}

// removePackages removes packages using the appropriate package manager
func removePackages(ctx context.Context, packages []string, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, distroConfig DistroConfig, repoDir string) error {
	if len(packages) <= 0 {
		return nil
	}

	logger.Log.Infof("Removing packages: %v", packages)

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "remove_packages")
	pkgJson, _ := json.Marshal(packages)
	span.SetAttributes(
		attribute.Int("remove_packages_count", len(packages)),
		attribute.String("remove_packages", string(pkgJson)),
	)
	defer span.End()

	useToolsChroot := toolsChroot != nil
	args := buildRemoveArgs(distroConfig, packages, useToolsChroot)
	err := executePackageManagerCommand(args, imageChroot, toolsChroot, useToolsChroot, distroConfig)
	if err != nil {
		return fmt.Errorf("failed to remove packages (%v):\n%w", packages, err)
	}
	return nil
}

// refreshPackageMetadata refreshes package metadata
func refreshPackageMetadata(ctx context.Context, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, distroConfig DistroConfig, repoDir string) error {
	logger.Log.Infof("Refreshing package metadata")

	useToolsChroot := toolsChroot != nil
	args := buildRefreshMetadataArgs(distroConfig, repoDir, useToolsChroot)
	err := executePackageManagerCommand(args, imageChroot, toolsChroot, useToolsChroot, distroConfig)
	if err != nil {
		return fmt.Errorf("failed to refresh package metadata:\n%w", err)
	}
	return nil
}

func cleanCache(imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, distroConfig DistroConfig) error {
	useToolsChroot := toolsChroot != nil
	args := buildCleanCacheArgs(distroConfig, useToolsChroot)
	err := executePackageManagerCommand(args, imageChroot, toolsChroot, useToolsChroot, distroConfig)
	if err != nil {
		return fmt.Errorf("failed to clean %s cache:\n%w", distroConfig.GetPackageManagerBinary(), err)
	}
	return nil
}

// addRemoveAndUpdatePackages orchestrates the complete package management workflow
func addRemoveAndUpdatePackages(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.OS,
	imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	rpmsSources []string, useBaseImageRpmRepos bool, distroConfig DistroConfig, snapshotTime string,
) error {
	var err error

	ctx, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "configure_packages")
	defer span.End()

	if snapshotTime == "" {
		snapshotTime = string(config.Packages.SnapshotTime)
	}

	packageManagerChroot := imageChroot
	if toolsChroot != nil {
		packageManagerChroot = toolsChroot
	}

	if distroConfig.GetDistroName() == "azurelinux" {
		err = createTempTdnfConfigWithSnapshot(packageManagerChroot, imagecustomizerapi.PackageSnapshotTime(snapshotTime))
		if err != nil {
			return err
		}
		defer func() {
			if cleanupErr := cleanupSnapshotTimeConfig(packageManagerChroot); cleanupErr != nil && err == nil {
				err = cleanupErr
			}
		}()
	}
	// Note: For Fedora, we use the existing generic config file without creating custom DNF config
	// Note: The 'validatePackageLists' function read the PackageLists files and merged them into the inline package lists.
	needPackageSources := len(config.Packages.Install) > 0 || len(config.Packages.Update) > 0 ||
		config.Packages.UpdateExistingPackages

	var mounts *rpmSourcesMounts
	if needPackageSources && distroConfig.GetPackageType() == "rpm" {
		// Mount RPM sources (only for RPM-based systems).
		mounts, err = mountRpmSources(ctx, buildDir, packageManagerChroot, rpmsSources, useBaseImageRpmRepos)
		if err != nil {
			return err
		}
		defer mounts.close()

		// Refresh metadata.
		err = refreshPackageMetadata(ctx, imageChroot, toolsChroot, distroConfig, distroConfig.GetPackageSourceDir())
		if err != nil {
			return err
		}

	} else if needPackageSources && distroConfig.GetPackageType() == "deb" {
		// Future: APT repository setup would go here
		// For APT-based systems, we would:
		// 1. Set up /etc/apt/sources.list or sources.list.d/
		// 2. Run apt-get update
		return fmt.Errorf("APT-based package management not yet implemented")
	}

	err = removePackages(ctx, config.Packages.Remove, imageChroot, toolsChroot, distroConfig, distroConfig.GetPackageSourceDir())
	if err != nil {
		return err
	}

	if config.Packages.UpdateExistingPackages {
		err = updateAllPackages(ctx, imageChroot, toolsChroot, distroConfig, distroConfig.GetPackageSourceDir())
		if err != nil {
			return err
		}
	}

	logger.Log.Infof("Installing packages: %v", config.Packages.Install)
	err = installOrUpdatePackages(ctx, "install", config.Packages.Install, imageChroot, toolsChroot, distroConfig, distroConfig.GetPackageSourceDir())
	if err != nil {
		return err
	}

	logger.Log.Infof("Updating packages: %v", config.Packages.Update)
	err = installOrUpdatePackages(ctx, "update", config.Packages.Update, imageChroot, toolsChroot, distroConfig, distroConfig.GetPackageSourceDir())
	if err != nil {
		return err
	}

	// Unmount package sources.
	if mounts != nil {
		err = mounts.close()
		if err != nil {
			return err
		}
	}

	if needPackageSources {
		err = cleanCache(imageChroot, toolsChroot, distroConfig)
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

// isPackageInstalled checks if a package is installed using the appropriate package manager
func isPackageInstalled(imageChroot safechroot.ChrootInterface, packageName string, distroName ...string) bool {
	var actualDistroName string
	if len(distroName) > 0 {
		actualDistroName = distroName[0]
	} else {
		actualDistroName = detectDistroName(imageChroot)
	}

	distroConfig := NewDistroConfig(actualDistroName)

	err := imageChroot.UnsafeRun(func() error {
		if distroConfig.GetPackageType() == "rpm" {
			_, _, err := shell.Execute(distroConfig.GetPackageManagerBinary(), "info", packageName, "--repo", "@system")
			return err
		} else {
			// Future: APT-based commands would go here
			// _, _, err := shell.Execute("dpkg", "-l", packageName)
			return fmt.Errorf("package type %s not yet supported for package check", distroConfig.GetPackageType())
		}
	})
	return err == nil
}
