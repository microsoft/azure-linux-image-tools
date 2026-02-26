package imagecustomizerlib

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
)

// managePackagesRpm provides a shared implementation for RPM-based package management
func managePackagesRpm(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.OS,
	imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, rpmsSources []string, disableBaseImageRpmRepos bool,
	snapshotTime imagecustomizerapi.PackageSnapshotTime, pmHandler rpmPackageManagerHandler,
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
		err = createTempTdnfConfigWithSnapshot(packageManagerChroot, snapshotTime)
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
		mounts, err = mountRpmSources(ctx, buildDir, packageManagerChroot, rpmsSources, disableBaseImageRpmRepos)
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
	err = installRpmPackages(ctx, config.Packages.Install, imageChroot, toolsChroot, pmHandler)
	if err != nil {
		return err
	}

	logger.Log.Infof("Updating packages: %v", config.Packages.Update)
	err = updateRpmPackages(ctx, config.Packages.Update, imageChroot, toolsChroot, pmHandler)
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
		err = cleanRpmCache(ctx, imageChroot, toolsChroot, pmHandler)
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

func installRpmPackages(ctx context.Context, allPackages []string,
	imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, pmHandler rpmPackageManagerHandler,
) error {
	if len(allPackages) == 0 {
		return nil
	}

	logger.Log.Infof("Installing packages (%d): %v", len(allPackages), allPackages)

	_, span := startInstallPackagesSpan(ctx, allPackages)
	defer span.End()

	// Build command arguments directly
	args := []string{pmHandler.getVerbosityOption(), "install", "--assumeyes", "--cacheonly"}

	args = append(args, "--setopt=reposdir="+rpmsMountParentDirInChroot)

	// Add package manager specific cache options (e.g., DNF cache metadata options)
	cacheOptions := pmHandler.getCacheOnlyOptions()
	args = append(args, cacheOptions...)

	args = append(args, allPackages...)

	if toolsChroot != nil {
		args = append([]string{
			"--releasever=" + pmHandler.getReleaseVersion(),
			"--installroot=/" + toolsRootImageDir,
		}, args...)
	}

	err := executeRpmPackageManagerCommand(args, imageChroot, toolsChroot, pmHandler)
	if err != nil {
		return fmt.Errorf("%w (%v):\n%w", ErrPackageInstall, allPackages, err)
	}
	return nil
}

func updateRpmPackages(ctx context.Context, allPackages []string,
	imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, pmHandler rpmPackageManagerHandler,
) error {
	if len(allPackages) == 0 {
		return nil
	}

	logger.Log.Infof("Updating packages (%d): %v", len(allPackages), allPackages)

	_, span := startUpdatePackagesSpan(ctx, allPackages)
	defer span.End()

	// Build command arguments directly
	args := []string{pmHandler.getVerbosityOption(), "update", "--assumeyes", "--cacheonly"}

	args = append(args, "--setopt=reposdir="+rpmsMountParentDirInChroot)

	// Add package manager specific cache options (e.g., DNF cache metadata options)
	cacheOptions := pmHandler.getCacheOnlyOptions()
	args = append(args, cacheOptions...)

	args = append(args, allPackages...)

	if toolsChroot != nil {
		args = append([]string{
			"--releasever=" + pmHandler.getReleaseVersion(),
			"--installroot=/" + toolsRootImageDir,
		}, args...)
	}

	err := executeRpmPackageManagerCommand(args, imageChroot, toolsChroot, pmHandler)
	if err != nil {
		return fmt.Errorf("%w (%v):\n%w", ErrPackageUpdate, allPackages, err)
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
	if len(allPackagesToRemove) <= 0 {
		return nil
	}

	logger.Log.Infof("Removing packages (%d): %v", len(allPackagesToRemove), allPackagesToRemove)

	_, span := startRemovePackagesSpan(ctx, allPackagesToRemove)
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
	logger.Log.Infof("Refreshing package metadata")

	_, span := startRefreshPackageMetadataSpan(ctx)
	defer span.End()

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

func cleanRpmCache(ctx context.Context, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	pmHandler rpmPackageManagerHandler,
) error {
	logger.Log.Infof("Cleaning RPM cache")

	_, span := startCleanPackagesCacheSpan(ctx)
	defer span.End()

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

// getAllPackagesFromChrootRpm retrieves all installed packages from an RPM-based system
func getAllPackagesFromChrootRpm(imageChroot safechroot.ChrootInterface) ([]OsPackage, error) {
	var out string
	err := imageChroot.UnsafeRun(func() error {
		var err error
		out, _, err = shell.Execute(
			"rpm", "-qa", "--queryformat", "%{NAME} %{VERSION} %{RELEASE} %{ARCH}\n",
		)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get RPM output from chroot:\n%w", err)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	var packages []OsPackage
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) != 4 {
			return nil, fmt.Errorf("malformed RPM line encountered while parsing installed RPMs for COSI: %q", line)
		}
		packages = append(packages, OsPackage{
			Name:    parts[0],
			Version: parts[1],
			Release: parts[2],
			Arch:    parts[3],
		})
	}

	return packages, nil
}
