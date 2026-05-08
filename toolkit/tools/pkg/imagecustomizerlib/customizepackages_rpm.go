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
)

// managePackagesRpm provides a shared implementation for RPM-based package management
func managePackagesRpm(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.OS,
	imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, rpmsSources []string, useBaseImageRpmRepos bool,
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

	var mounts *rpmSourcesMounts
	if needPackageSources(config) {
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
		err = updateExistingRpmPackages(ctx, imageChroot, toolsChroot, pmHandler)
		if err != nil {
			return err
		}
	}

	err = installRpmPackages(ctx, config.Packages.Install, imageChroot, toolsChroot, pmHandler)
	if err != nil {
		return err
	}

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

	if needPackageSources(config) {
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

	return shell.NewExecBuilder(pmHandler.getPackageManagerBinary(), args...).
		StdoutCallback(stdoutCallback).
		LogLevel(shell.LogDisabledLevel, logrus.DebugLevel).
		ErrorStderrLines(1).
		Chroot(pmChroot.ChrootDir()).
		Execute()
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

// updateExistingRpmPackages updates existing packages using the appropriate package manager
func updateExistingRpmPackages(ctx context.Context, imageChroot *safechroot.Chroot,
	toolsChroot *safechroot.Chroot, pmHandler rpmPackageManagerHandler,
) error {
	logger.Log.Infof("Updating existing packages")

	_, span := startUpdateExistingPackagesSpan(ctx)
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

	// --setopt=skip_if_unavailable=False ensures failures to fetch repo metadata are fatal. TDNF already does this by
	// default, but DNF on some distros (e.g. Azure Linux 4.0) defaults to skip_if_unavailable=True, so we set it
	// explicitly for both. It is important to ensure metadata is refreshed successfully to ensure correctness of
	// package install operations and to catch any typos in user-provided repository configuration. Otherwise, the wrong
	// package versions might be silently installed.
	args := []string{
		pmHandler.getVerbosityOption(), "check-update", "--refresh", "--assumeyes",
		"--setopt=skip_if_unavailable=False",
	}

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
	out, _, err := shell.NewExecBuilder("rpm", "-qa", "--queryformat", "%{NAME} %{VERSION} %{RELEASE} %{ARCH}\n").
		LogLevel(logrus.TraceLevel, logrus.DebugLevel).
		Chroot(imageChroot.ChrootDir()).
		ExecuteCaptureOutput()
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
