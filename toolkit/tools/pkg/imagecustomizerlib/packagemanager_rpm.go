// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// executePackageManagerCommand runs a package manager command with proper chroot handling
func executePackageManagerCommand(args []string, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, pmHandler rpmPackageManagerHandler) error {
	pmChroot := imageChroot
	if toolsChroot != nil {
		pmChroot = toolsChroot
	}

	if _, err := os.Stat(filepath.Join(pmChroot.RootDir(), pmHandler.getConfigFile())); err == nil {
		args = append([]string{"--config", "/" + pmHandler.getConfigFile()}, args...)
	}

	// Use package manager specific output callback
	stdoutCallback := pmHandler.createOutputCallback()

	return pmChroot.UnsafeRun(func() error {
		return shell.NewExecBuilder(pmHandler.getPackageManagerBinary(), args...).
			StdoutCallback(stdoutCallback).
			LogLevel(shell.LogDisabledLevel, logrus.DebugLevel).
			ErrorStderrLines(1).
			Execute()
	})
}

func installOrUpdatePackages(ctx context.Context, action string, allPackagesToAdd []string, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, pmHandler rpmPackageManagerHandler) error {
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

	repoDir := pmHandler.getPackageSourceDir()
	if repoDir != "" {
		args = append(args, "--setopt=reposdir="+repoDir)
	}

	args = append(args, allPackagesToAdd...)

	if toolsChroot != nil {
		args = append([]string{"--releasever=" + pmHandler.getReleaseVersion(), "--installroot=/" + toolsRootImageDir}, args...)
	}

	err := executePackageManagerCommand(args, imageChroot, toolsChroot, pmHandler)
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
func updateAllPackages(ctx context.Context, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, pmHandler rpmPackageManagerHandler) error {
	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "update_base_packages")
	defer span.End()

	// Build command arguments directly
	args := []string{"-v", "update", "--assumeyes", "--cacheonly"}

	repoDir := pmHandler.getPackageSourceDir()
	if repoDir != "" {
		args = append(args, "--setopt=reposdir="+repoDir)
	}

	if toolsChroot != nil {
		args = append([]string{"--releasever=" + pmHandler.getReleaseVersion(), "--installroot=/" + toolsRootImageDir}, args...)
	}

	err := executePackageManagerCommand(args, imageChroot, toolsChroot, pmHandler)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrPackagesUpdateInstalled, err)
	}
	return nil
}

// removePackages removes packages using the appropriate package manager
func removePackages(ctx context.Context, allPackagesToRemove []string, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, pmHandler rpmPackageManagerHandler) error {
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
		args = append([]string{"--releasever=" + pmHandler.getReleaseVersion(), "--installroot=/" + toolsRootImageDir}, args...)
	}

	err := executePackageManagerCommand(args, imageChroot, toolsChroot, pmHandler)
	if err != nil {
		return fmt.Errorf("%w (%v):\n%w", ErrPackageRemove, allPackagesToRemove, err)
	}
	return nil
}

// refreshPackageMetadata refreshes package metadata
func refreshPackageMetadata(ctx context.Context, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, pmHandler rpmPackageManagerHandler) error {
	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "refresh_metadata")
	defer span.End()

	logger.Log.Infof("Refreshing package metadata")

	args := []string{
		"-v", "check-update", "--refresh", "--assumeyes",
	}

	repoDir := pmHandler.getPackageSourceDir()
	if repoDir != "" {
		args = append(args, "--setopt=reposdir="+repoDir)
	}

	if toolsChroot != nil {
		args = append([]string{"--releasever=" + pmHandler.getReleaseVersion(), "--installroot=/" + toolsRootImageDir}, args...)
	}

	err := executePackageManagerCommand(args, imageChroot, toolsChroot, pmHandler)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrPackageRepoMetadataRefresh, err)
	}
	return nil
}

func cleanCache(imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, pmHandler rpmPackageManagerHandler) error {
	// Build command arguments directly
	args := []string{"-v", "clean", "all"}

	if toolsChroot != nil {
		args = append([]string{"--releasever=" + pmHandler.getReleaseVersion(), "--installroot=/" + toolsRootImageDir}, args...)
	}

	err := executePackageManagerCommand(args, imageChroot, toolsChroot, pmHandler)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrPackageCacheClean, err)
	}
	return nil
}

// prefillDnfCache downloads packages to DNF cache for offline installation
func prefillDnfCache(ctx context.Context, installPackages []string, updatePackages []string, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, pmHandler rpmPackageManagerHandler) error {
	allPackages := append(installPackages, updatePackages...)
	if len(allPackages) == 0 {
		return nil
	}

	logger.Log.Infof("Pre-filling DNF cache for packages: %v", allPackages)

	args := []string{"-v", "install", "--assumeyes", "--downloadonly"}

	repoDir := pmHandler.getPackageSourceDir()
	if repoDir != "" {
		args = append(args, "--setopt=reposdir="+repoDir)
	}

	// Enable keepcache to ensure packages are stored in cache
	args = append(args, "--setopt=keepcache=1")

	if toolsChroot != nil {
		args = append([]string{"--releasever=" + pmHandler.getReleaseVersion(), "--installroot=/" + toolsRootImageDir}, args...)
	}

	args = append(args, allPackages...)

	err := executePackageManagerCommand(args, imageChroot, toolsChroot, pmHandler)
	if err != nil {
		return fmt.Errorf("failed to prefill DNF cache for packages (%v):\n%w", allPackages, err)
	}
	return nil
}
