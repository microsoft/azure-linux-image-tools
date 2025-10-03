package imagecustomizerlib

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// managePackagesRpm provides a shared implementation for RPM-based package management
func managePackagesRpm(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.OS,
	imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, rpmsSources []string, useBaseImageRpmRepos bool,
	snapshotTime string, pmHandler rpmPackageManagerHandler,
) error {
	var err error

	packageManagerChroot := imageChroot
	if toolsChroot != nil {
		packageManagerChroot = toolsChroot
	}

	// Validate that snapshot time is only used with package managers that support it
	if snapshotTime != "" && !pmHandler.supportsSnapshotTime() {
		return fmt.Errorf("%w: package manager %s does not support snapshot time",
			ErrSnapshotTimeNotSupported, pmHandler.getPackageManagerBinary())
	}

	// Setup distribution-specific configuration if needed
	if pmHandler.supportsSnapshotTime() && snapshotTime != "" {
		// Setup Azure Linux specific TDNF configuration with snapshot
		err = createTempTdnfConfigWithSnapshot(
			packageManagerChroot,
			imagecustomizerapi.PackageSnapshotTime(snapshotTime),
		)
		if err != nil {
			return err
		}
		defer func() {
			if cleanupErr := cleanupSnapshotTimeConfig(packageManagerChroot); cleanupErr != nil && err == nil {
				err = cleanupErr
			}
		}()
	}

	needPackageSources := len(config.Packages.Install) > 0 || len(config.Packages.Update) > 0 ||
		config.Packages.UpdateExistingPackages

	var mounts *rpmSourcesMounts
	if needPackageSources {
		// Mount RPM sources
		mounts, err = mountRpmSources(ctx, buildDir, packageManagerChroot, rpmsSources, useBaseImageRpmRepos)
		if err != nil {
			return err
		}
		defer mounts.close()

		// Refresh metadata
		err = refreshRpmPackageMetadata(ctx, imageChroot, toolsChroot, pmHandler)
		if err != nil {
			return err
		}

	}

	// Execute package operations
	err = removeRpmPackages(ctx, config.Packages.Remove, imageChroot, toolsChroot, pmHandler)
	if err != nil {
		return err
	}

	if config.Packages.UpdateExistingPackages {
		err = updateAllRpmPackages(ctx, imageChroot, toolsChroot, pmHandler)
		if err != nil {
			return err
		}
	}

	logger.Log.Infof("Installing packages: %v", config.Packages.Install)
	err = installOrUpdateRpmPackages(ctx, "install", config.Packages.Install, imageChroot, toolsChroot, pmHandler)
	if err != nil {
		return err
	}

	logger.Log.Infof("Updating packages: %v", config.Packages.Update)
	err = installOrUpdateRpmPackages(ctx, "update", config.Packages.Update, imageChroot, toolsChroot, pmHandler)
	if err != nil {
		return err
	}

	// Cleanup
	if mounts != nil {
		err = mounts.close()
		if err != nil {
			return err
		}
	}

	if needPackageSources {
		err = cleanRpmCache(imageChroot, toolsChroot, pmHandler)
		if err != nil {
			return err
		}
	}

	return nil
}

// executeRpmPackageManagerCommand runs a package manager command with proper chroot handling
func executeRpmPackageManagerCommand(args []string, imageChroot *safechroot.Chroot,
	toolsChroot *safechroot.Chroot, pmHandler rpmPackageManagerHandler,
) error {
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

func installOrUpdateRpmPackages(ctx context.Context, action string, allPackagesToAdd []string,
	imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, pmHandler rpmPackageManagerHandler,
) error {
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
	args := []string{pmHandler.getVerbosityOption(), action, "--assumeyes", "--cacheonly"}

	args = append(args, "--setopt=reposdir="+rpmsMountParentDirInChroot)

	// Add package manager specific cache options (e.g., DNF cache metadata options)
	cacheOptions := pmHandler.getCacheOnlyOptions()
	args = append(args, cacheOptions...)

	args = append(args, allPackagesToAdd...)

	if toolsChroot != nil {
		args = append([]string{
			"--releasever=" + pmHandler.getReleaseVersion(),
			"--installroot=/" + toolsRootImageDir,
		}, args...)
	}

	err := executeRpmPackageManagerCommand(args, imageChroot, toolsChroot, pmHandler)
	if err != nil {
		if action == "install" {
			return fmt.Errorf("%w (%v):\n%w", ErrPackageInstall, allPackagesToAdd, err)
		} else {
			return fmt.Errorf("%w (%v):\n%w", ErrPackageUpdate, allPackagesToAdd, err)
		}
	}
	return nil
}

// updateAllRpmPackages updates all packages using the appropriate package manager
func updateAllRpmPackages(ctx context.Context, imageChroot *safechroot.Chroot,
	toolsChroot *safechroot.Chroot, pmHandler rpmPackageManagerHandler,
) error {
	logger.Log.Infof("Updating base image packages")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "update_base_packages")
	defer span.End()

	// Build command arguments directly
	args := []string{pmHandler.getVerbosityOption(), "update", "--assumeyes", "--cacheonly"}

	args = append(args, "--setopt=reposdir="+rpmsMountParentDirInChroot)

	// Add package manager specific cache options (e.g., DNF cache metadata options)
	cacheOptions := pmHandler.getCacheOnlyOptions()
	args = append(args, cacheOptions...)

	if toolsChroot != nil {
		args = append([]string{
			"--releasever=" + pmHandler.getReleaseVersion(),
			"--installroot=/" + toolsRootImageDir,
		}, args...)
	}

	err := executeRpmPackageManagerCommand(args, imageChroot, toolsChroot, pmHandler)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrPackagesUpdateInstalled, err)
	}
	return nil
}

// removeRpmPackages removes packages using the appropriate package manager
func removeRpmPackages(ctx context.Context, allPackagesToRemove []string, imageChroot *safechroot.Chroot,
	toolsChroot *safechroot.Chroot, pmHandler rpmPackageManagerHandler,
) error {
	logger.Log.Infof("Removing packages: %v", allPackagesToRemove)

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
	args := []string{pmHandler.getVerbosityOption(), "remove", "--assumeyes", "--disablerepo", "*"}
	args = append(args, allPackagesToRemove...)

	if toolsChroot != nil {
		args = append(
			[]string{"--releasever=" + pmHandler.getReleaseVersion(), "--installroot=/" + toolsRootImageDir},
			args...)
	}

	err := executeRpmPackageManagerCommand(args, imageChroot, toolsChroot, pmHandler)
	if err != nil {
		return fmt.Errorf("%w (%v):\n%w", ErrPackageRemove, allPackagesToRemove, err)
	}
	return nil
}

// refreshRpmPackageMetadata refreshes package metadata
func refreshRpmPackageMetadata(ctx context.Context, imageChroot *safechroot.Chroot,
	toolsChroot *safechroot.Chroot, pmHandler rpmPackageManagerHandler,
) error {
	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "refresh_metadata")
	defer span.End()

	logger.Log.Infof("Refreshing package metadata")

	args := []string{pmHandler.getVerbosityOption(), "check-update", "--refresh", "--assumeyes"}

	args = append(args, "--setopt=reposdir="+rpmsMountParentDirInChroot)

	if toolsChroot != nil {
		args = append([]string{
			"--releasever=" + pmHandler.getReleaseVersion(),
			"--installroot=/" + toolsRootImageDir,
		}, args...)
	}

	err := executeRpmPackageManagerCommand(args, imageChroot, toolsChroot, pmHandler)
	if err != nil {
		// For DNF/TDNF check-update, exit code 100 means updates are available
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 100 {
			logger.Log.Debugf("Package updates are available (exit code 100)")
			return nil
		}
		return fmt.Errorf("%w:\n%w", ErrPackageRepoMetadataRefresh, err)
	}
	return nil
}

func cleanRpmCache(imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	pmHandler rpmPackageManagerHandler,
) error {
	logger.Log.Infof("Cleaning up RPM cache")
	// Build command arguments directly
	args := []string{pmHandler.getVerbosityOption(), "clean", "all"}

	if toolsChroot != nil {
		args = append([]string{
			"--releasever=" + pmHandler.getReleaseVersion(),
			"--installroot=/" + toolsRootImageDir,
		}, args...)
	}

	err := executeRpmPackageManagerCommand(args, imageChroot, toolsChroot, pmHandler)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrPackageCacheClean, err)
	}
	return nil
}
