// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
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

var (
	// Package-related errors
	ErrPackageRepoMetadataRefresh = NewImageCustomizerError("Packages:RepoMetadataRefresh", "failed to refresh tdnf repo metadata")
	ErrInvalidPackageListFile     = NewImageCustomizerError("Packages:InvalidPackageListFile", "failed to read package list file")
	ErrPackageRemove              = NewImageCustomizerError("Packages:Remove", "failed to remove packages")
	ErrPackageUpdate              = NewImageCustomizerError("Packages:Update", "failed to update packages")
	ErrPackagesUpdateInstalled    = NewImageCustomizerError("Packages:UpdateInstalled", "failed to update installed packages")
	ErrPackageInstall             = NewImageCustomizerError("Packages:Install", "failed to install packages")
	ErrPackageCacheClean          = NewImageCustomizerError("Packages:CacheClean", "failed to clean tdnf cache")
	ErrMountRpmSources            = NewImageCustomizerError("Packages:MountRpmSources", "failed to mount RPM sources")
)

// executePackageManagerCommand runs a package manager command with proper chroot handling
func executePackageManagerCommand(args []string, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, distroConfig distroHandler) error {
	pmChroot := imageChroot
	if toolsChroot != nil {
		pmChroot = toolsChroot
	}

	if _, err := os.Stat(filepath.Join(pmChroot.RootDir(), distroConfig.getConfigFile())); err == nil {
		args = append([]string{"--config", "/" + distroConfig.getConfigFile()}, args...)
	}

	// Use distribution-specific output callback
	stdoutCallback := distroConfig.createOutputCallback()

	return pmChroot.UnsafeRun(func() error {
		return shell.NewExecBuilder(distroConfig.getPackageManagerBinary(), args...).
			StdoutCallback(stdoutCallback).
			LogLevel(shell.LogDisabledLevel, logrus.DebugLevel).
			ErrorStderrLines(1).
			Execute()
	})
}

func installOrUpdatePackages(ctx context.Context, action string, allPackagesToAdd []string, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, distroConfig distroHandler) error {
	if len(allPackagesToAdd) == 0 {
		return nil
	}

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, action+"_packages")
	span.SetAttributes(
		attribute.Int(action+"_packages_count", len(allPackagesToAdd)),
		attribute.StringSlice("packages", allPackagesToAdd),
	)
	defer span.End()

	// Build command arguments directly
	args := []string{"-v", action, "--assumeyes", "--cacheonly"}

	repoDir := distroConfig.getPackageSourceDir()
	if repoDir != "" {
		args = append(args, "--setopt=reposdir="+repoDir)
	}

	args = append(args, allPackagesToAdd...)

	if toolsChroot != nil {
		args = append([]string{"--releasever=" + distroConfig.getReleaseVersion(), "--installroot=/" + toolsRootImageDir}, args...)
	}

	err := executePackageManagerCommand(args, imageChroot, toolsChroot, distroConfig)
	if err != nil {
		if action == "install" {
			return fmt.Errorf("%w (%v):\n%w", ErrPackageInstall, allPackagesToAdd, err)
		} else {
			return fmt.Errorf("%w (%v):\n%w", ErrPackageUpdate, allPackagesToAdd, err)
		}
	}
	return nil
}

// updateAllPackages updates all packages using the appropriate package manager
func updateAllPackages(ctx context.Context, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, distroConfig distroHandler) error {
	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "update_base_packages")
	defer span.End()

	// Build command arguments directly
	args := []string{"-v", "update", "--assumeyes", "--cacheonly"}

	repoDir := distroConfig.getPackageSourceDir()
	if repoDir != "" {
		args = append(args, "--setopt=reposdir="+repoDir)
	}

	if toolsChroot != nil {
		args = append([]string{"--releasever=" + distroConfig.getReleaseVersion(), "--installroot=/" + toolsRootImageDir}, args...)
	}

	err := executePackageManagerCommand(args, imageChroot, toolsChroot, distroConfig)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrPackagesUpdateInstalled, err)
	}
	return nil
}

// removePackages removes packages using the appropriate package manager
func removePackages(ctx context.Context, allPackagesToRemove []string, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, distroConfig distroHandler) error {
	if len(allPackagesToRemove) <= 0 {
		return nil
	}

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "remove_packages")
	span.SetAttributes(
		attribute.Int("remove_packages_count", len(allPackagesToRemove)),
		attribute.StringSlice("remove_packages", allPackagesToRemove),
	)
	defer span.End()

	// Build command arguments directly
	args := []string{"-v", "remove", "--assumeyes", "--disablerepo", "*"}
	args = append(args, allPackagesToRemove...)

	if toolsChroot != nil {
		args = append([]string{"--releasever=" + distroConfig.getReleaseVersion(), "--installroot=/" + toolsRootImageDir}, args...)
	}

	err := executePackageManagerCommand(args, imageChroot, toolsChroot, distroConfig)
	if err != nil {
		return fmt.Errorf("%w (%v):\n%w", ErrPackageRemove, allPackagesToRemove, err)
	}
	return nil
}

// refreshPackageMetadata refreshes package metadata
func refreshPackageMetadata(ctx context.Context, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, distroConfig distroHandler) error {
	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "refresh_metadata")
	defer span.End()

	logger.Log.Infof("Refreshing package metadata")

	args := []string{
		"-v", "check-update", "--refresh", "--assumeyes",
	}

	repoDir := distroConfig.getPackageSourceDir()
	if repoDir != "" {
		args = append(args, "--setopt=reposdir="+repoDir)
	}

	if toolsChroot != nil {
		args = append([]string{"--releasever=" + distroConfig.getReleaseVersion(), "--installroot=/" + toolsRootImageDir}, args...)
	}

	err := executePackageManagerCommand(args, imageChroot, toolsChroot, distroConfig)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrPackageRepoMetadataRefresh, err)
	}
	return nil
}

func cleanCache(imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, distroConfig distroHandler) error {
	// Build command arguments directly
	args := []string{"-v", "clean", "all"}

	if toolsChroot != nil {
		args = append([]string{"--releasever=" + distroConfig.getReleaseVersion(), "--installroot=/" + toolsRootImageDir}, args...)
	}

	err := executePackageManagerCommand(args, imageChroot, toolsChroot, distroConfig)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrPackageCacheClean, err)
	}
	return nil
}

// addRemoveAndUpdatePackages orchestrates the complete package management workflow
func addRemoveAndUpdatePackages(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.OS,
	imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	rpmsSources []string, useBaseImageRpmRepos bool, distroConfig distroHandler, snapshotTime string,
) error {
	ctx, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "configure_packages")
	defer span.End()

	if snapshotTime == "" {
		snapshotTime = string(config.Packages.SnapshotTime)
	}

	// Delegate the entire package management workflow to the distribution-specific implementation
	return distroConfig.managePackages(ctx, buildDir, baseConfigPath, config, imageChroot, toolsChroot, rpmsSources, useBaseImageRpmRepos, snapshotTime)
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
			return nil, fmt.Errorf("%w (%s):\n%w", ErrInvalidPackageListFile, packageListFilePath, err)
		}

		allPackages = append(allPackages, packageList.Packages...)
	}

	allPackages = append(allPackages, packages...)
	return allPackages, nil
}

// TODO remove this after adding fedora support for image customizer
func isPackageInstalled(imageChroot safechroot.ChrootInterface, packageName string) bool {
	err := imageChroot.UnsafeRun(func() error {
		_, _, err := shell.Execute("tdnf", "info", packageName, "--repo", "@system")
		return err
	})
	if err != nil {
		return false
	}
	return true
}
