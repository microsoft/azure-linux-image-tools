package imagecustomizerlib

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/cosiapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/sliceutils"
	"github.com/sirupsen/logrus"
	"github.com/spdx/tools-golang/spdx"
)

// managePackagesRpm provides a shared implementation for RPM-based package management
func managePackagesRpm(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.OS,
	imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, rpmsSources []string, useBaseImageRpmRepos bool,
	snapshotTime imagecustomizerapi.PackageSnapshotTime, pmHandler rpmPackageManagerHandler,
) (err error) {
	packageManagerChroot := imageChroot
	if toolsChroot != nil {
		packageManagerChroot = toolsChroot
	}

	if snapshotTime != "" {
		var cleanup func() error
		cleanup, err = pmHandler.configureSnapshotTime(packageManagerChroot, snapshotTime)
		if err != nil {
			return err
		}
		defer func() {
			if cleanupErr := cleanup(); cleanupErr != nil && err == nil {
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
		err = refreshRpmPackageMetadata(ctx, imageChroot, toolsChroot, pmHandler, mounts.chrootGpgKeys,
			mounts.uriGpgKeys)
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
	args := []string{"install", "--assumeyes", "--cacheonly"}

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

	_, _, err := pmHandler.executeCommand(args, imageChroot, toolsChroot)
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
	args := []string{"update", "--assumeyes", "--cacheonly"}

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

	_, _, err := pmHandler.executeCommand(args, imageChroot, toolsChroot)
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
	args := []string{"update", "--assumeyes", "--cacheonly"}

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

	_, _, err := pmHandler.executeCommand(args, imageChroot, toolsChroot)
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
	args := []string{"remove", "--assumeyes", "--disablerepo", "*"}
	args = append(args, allPackagesToRemove...)

	if toolsChroot != nil {
		args = append(
			[]string{"--releasever=" + pmHandler.getReleaseVersion(), "--installroot=/" + toolsRootImageDir},
			args...)
	}

	_, _, err := pmHandler.executeCommand(args, imageChroot, toolsChroot)
	if err != nil {
		return fmt.Errorf("%w (%v):\n%w", ErrPackageRemove, allPackagesToRemove, err)
	}
	return nil
}

// refreshRpmPackageMetadata refreshes package metadata
func refreshRpmPackageMetadata(ctx context.Context, imageChroot *safechroot.Chroot,
	toolsChroot *safechroot.Chroot, pmHandler rpmPackageManagerHandler, chrootGpgKeys []string, uriGpgKeys []string,
) error {
	logger.Log.Infof("Refreshing package metadata")

	_, span := startRefreshPackageMetadataSpan(ctx)
	defer span.End()

	err := pmHandler.importGpgKeys(imageChroot, toolsChroot, chrootGpgKeys, uriGpgKeys)
	if err != nil {
		return err
	}

	// --setopt=skip_if_unavailable=False ensures failures to fetch repo metadata are fatal. TDNF already does this by
	// default, but DNF on some distros (e.g. Azure Linux 4.0) defaults to skip_if_unavailable=True, so we set it
	// explicitly for both. It is important to ensure metadata is refreshed successfully to ensure correctness of
	// package install operations and to catch any typos in user-provided repository configuration. Otherwise, the wrong
	// package versions might be silently installed.
	args := []string{
		"check-update", "--refresh", "--assumeyes",
		"--setopt=skip_if_unavailable=False",
	}

	args = append(args, "--setopt=reposdir="+rpmsMountParentDirInChroot)

	if toolsChroot != nil {
		args = append([]string{
			"--releasever=" + pmHandler.getReleaseVersion(),
			"--installroot=/" + toolsRootImageDir,
		}, args...)
	}

	_, _, err = pmHandler.executeCommand(args, imageChroot, toolsChroot)
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
	args := []string{"clean", "all"}

	if toolsChroot != nil {
		args = append([]string{
			"--releasever=" + pmHandler.getReleaseVersion(),
			"--installroot=/" + toolsRootImageDir,
		}, args...)
	}

	_, _, err := pmHandler.executeCommand(args, imageChroot, toolsChroot)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrPackageCacheClean, err)
	}
	return nil
}

// getAllPackagesFromChrootRpm retrieves all installed packages from an RPM-based system
func getAllPackagesFromChrootRpm(imageChroot safechroot.ChrootInterface, toolsChroot *safechroot.Chroot,
) ([]cosiapi.OsPackage, error) {
	args := []string{"-qa", "--queryformat", "%{NAME} %{VERSION} %{RELEASE} %{ARCH}\n"}

	chroot := imageChroot
	if toolsChroot != nil {
		// Run rpm from inside the tools chroot against the image bind-mounted at /_imageroot — needed when
		// imageChroot has no in-image rpm.
		args = append([]string{"--root", "/" + toolsRootImageDir}, args...)
		chroot = toolsChroot
	}

	out, _, err := shell.NewExecBuilder("rpm", args...).
		LogLevel(logrus.TraceLevel, logrus.DebugLevel).
		Chroot(chroot.ChrootDir()).
		ExecuteCaptureOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get RPM output from chroot:\n%w", err)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	var packages []cosiapi.OsPackage
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) != 4 {
			return nil, fmt.Errorf("malformed RPM line encountered while parsing installed RPMs for COSI: %q", line)
		}
		packages = append(packages, cosiapi.OsPackage{
			Name:    parts[0],
			Version: parts[1],
			Release: parts[2],
			Arch:    parts[3],
		})
	}

	return packages, nil
}

func rpmGetOsManifestPackages(imageChroot safechroot.ChrootInterface, toolsChroot *safechroot.Chroot,
) (osManifestPackages, error) {
	args := []string{"-qa", "--queryformat", "%{NAME} %{EVR} %{SHA256HEADER}\n"}

	chroot := imageChroot
	if toolsChroot != nil {
		// Run rpm from inside the tools chroot against the image bind-mounted at /_imageroot — needed when
		// imageChroot has no in-image rpm.
		args = append([]string{"--root", "/" + toolsRootImageDir}, args...)
		chroot = toolsChroot
	}

	out, _, err := shell.NewExecBuilder("rpm", args...).
		LogLevel(logrus.TraceLevel, logrus.DebugLevel).
		Chroot(chroot.ChrootDir()).
		ExecuteCaptureOutput()
	if err != nil {
		return osManifestPackages{}, fmt.Errorf("failed to get RPM output from chroot:\n%w", err)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	var packages []*spdx.Package
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) != 3 {
			return osManifestPackages{}, fmt.Errorf("malformed RPM line encountered while parsing installed RPMs: %q", line)
		}

		name := parts[0]
		version := parts[1]
		signature := parts[2]

		packages = append(packages, &spdx.Package{
			PackageName:             name,
			PackageSPDXIdentifier:   spdx.ElementID(fmt.Sprintf("Package-%s", signature)),
			PackageVersion:          version,
			PackageDownloadLocation: "NOASSERTION",
			FilesAnalyzed:           false,
			PackageLicenseConcluded: "NOASSERTION",
			PackageLicenseDeclared:  "NOASSERTION",
			PackageCopyrightText:    "NOASSERTION",
			PackageSupplier: &spdx.Supplier{
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

func rpmRemovePackageManagerTools(imageChroot *safechroot.Chroot, pmHandler rpmPackageManagerHandler,
	toolsChroot *safechroot.Chroot, packageManagementPackages []string,
) (osManifestPackages, error) {
	var err error

	manifest := osManifestPackages{}
	if toolsChroot == nil {
		// Once the package manager tools have been removed, it will no longer be possible to collect the package list.
		// So, collect it now.
		manifest, err = rpmGetOsManifestPackages(imageChroot, toolsChroot)
		if err != nil {
			return osManifestPackages{}, fmt.Errorf("%w:\n%w", ErrCollectManifestPackages, err)
		}
	}

	removedPackages, err := rpmEnsurePackagesRemoved(imageChroot, pmHandler, toolsChroot, packageManagementPackages,
		true /*removeProtectedPackages*/)
	if err != nil {
		return osManifestPackages{}, err
	}

	if toolsChroot == nil {
		// Remove the packages that were just removed from the list of installed packages.
		removedPackagesSet := sliceutils.SliceToSet(removedPackages)

		manifest.Filter(func(packageInfo *spdx.Package) bool {
			_, packageRemoved := removedPackagesSet[pkgNameAndVersion{packageInfo.PackageName, packageInfo.PackageVersion}]
			return !packageRemoved
		})
	} else {
		// Collect the list of installed packages.
		// Note: Log parsing is a little brittle. So, when the tools directory is available, it is more robust to query
		// the list of packages after the tools have been removed.
		manifest, err = rpmGetOsManifestPackages(imageChroot, toolsChroot)
		if err != nil {
			return osManifestPackages{}, fmt.Errorf("%w:\n%w", ErrCollectManifestPackages, err)
		}
	}

	return manifest, nil
}

func rpmEnsurePackagesRemoved(imageChroot *safechroot.Chroot, pmHandler rpmPackageManagerHandler,
	toolsChroot *safechroot.Chroot, packages []string, removeProtectedPackages bool,
) ([]pkgNameAndVersion, error) {
	packagesToRemove := []string(nil)
	for _, packageName := range packages {
		installed, err := pmHandler.isPackageInstalled(imageChroot, toolsChroot, packageName)
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

	args := []string{"--assumeyes", "--disablerepo", "*"}
	if removeProtectedPackages {
		args = append(args, "--setopt=protected_packages=")
	}
	args = append(args, "remove")
	args = append(args, packagesToRemove...)

	if toolsChroot != nil {
		args = append([]string{
			"--releasever=" + pmHandler.getReleaseVersion(),
			"--installroot=/" + toolsRootImageDir,
		}, args...)
	}

	_, removedPackages, err := pmHandler.executeCommand(args, imageChroot, toolsChroot)
	if err != nil {
		return nil, fmt.Errorf("%w (%v):\n%w", ErrPackageRemove, packagesToRemove, err)
	}

	return removedPackages, nil
}
