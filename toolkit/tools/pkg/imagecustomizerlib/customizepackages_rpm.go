package imagecustomizerlib

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	rpmdb "github.com/anchore/go-rpmdb/pkg"
	_ "github.com/glebarez/go-sqlite"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/cosiapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/purl"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/spdxmanifest"
	"github.com/sirupsen/logrus"
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

	args = append(getRpmInstallRootArgs(pmHandler, toolsChroot), args...)

	err := pmHandler.executeCommand(args, imageChroot, toolsChroot)
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

	args = append(getRpmInstallRootArgs(pmHandler, toolsChroot), args...)

	err := pmHandler.executeCommand(args, imageChroot, toolsChroot)
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

	args = append(getRpmInstallRootArgs(pmHandler, toolsChroot), args...)

	err := pmHandler.executeCommand(args, imageChroot, toolsChroot)
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

	args := getRpmRemoveArgs(pmHandler, toolsChroot, allPackagesToRemove,
		false /* removeProtectedPackages */)

	err := pmHandler.executeCommand(args, imageChroot, toolsChroot)
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

	args = append(getRpmInstallRootArgs(pmHandler, toolsChroot), args...)

	err = pmHandler.executeCommand(args, imageChroot, toolsChroot)
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

	args = append(getRpmInstallRootArgs(pmHandler, toolsChroot), args...)

	err := pmHandler.executeCommand(args, imageChroot, toolsChroot)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrPackageCacheClean, err)
	}
	return nil
}

// getAllPackagesFromChrootRpm retrieves all installed packages from an RPM-based system
func getAllPackagesFromChrootRpm(imageChroot safechroot.ChrootInterface, toolsChroot *safechroot.Chroot,
) ([]cosiapi.OsPackage, error) {
	args := []string{"-qa", "--queryformat", "%{NAME} %{VERSION} %{RELEASE} %{ARCH}\n"}

	args = append(getRpmRootArgs(toolsChroot), args...)
	chroot := getRpmChroot(imageChroot, toolsChroot)

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

func getRpmChroot(imageChroot safechroot.ChrootInterface, toolsChroot *safechroot.Chroot,
) safechroot.ChrootInterface {
	if toolsChroot == nil {
		return imageChroot
	}

	return toolsChroot
}

func listInstalledPackagesRpm(imageChroot safechroot.ChrootInterface, rpmDatabasePath string, purlNamespace string,
) ([]spdxmanifest.Package, error) {
	databasePath := filepath.Join(imageChroot.RootDir(), rpmDatabasePath)

	walInfo, err := os.Stat(databasePath + "-wal")
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err == nil && walInfo.Size() != 0 {
		return nil, fmt.Errorf("cannot read RPM database with a nonempty WAL (path='%s')", databasePath)
	}

	database, err := rpmdb.Open(databasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open RPM database (path='%s'):\n%w", databasePath, err)
	}
	defer database.Close()

	packages, err := database.ListPackages()
	if err != nil {
		return nil, fmt.Errorf("failed to list RPM database packages (path='%s'):\n%w", databasePath, err)
	}
	if err := database.Close(); err != nil {
		return nil, fmt.Errorf("failed to close RPM database (path='%s'):\n%w", databasePath, err)
	}

	installedPackages := make([]spdxmanifest.Package, 0, len(packages))
	installedIds := make(map[string]struct{}, len(packages))
	for _, packageInfo := range packages {
		if packageInfo.Arch == "" {
			logger.Log.Debugf("Skipping RPM record without architecture (name='%s', version='%s', release='%s')",
				packageInfo.Name, packageInfo.Version, packageInfo.Release)
			continue
		}
		manifestPackage, err := newManifestPackageFromRpmPackage(*packageInfo, purlNamespace)
		if err != nil {
			return nil, err
		}

		if _, found := installedIds[manifestPackage.ID]; found {
			logger.Log.Warnf("Duplicate installed package ID (id='%s')", manifestPackage.ID)
		}
		installedIds[manifestPackage.ID] = struct{}{}
		installedPackages = append(installedPackages, manifestPackage)
	}
	return installedPackages, nil
}

func newManifestPackageFromRpmPackage(rpmPackage rpmdb.PackageInfo, namespace string) (spdxmanifest.Package, error) {
	if rpmPackage.Name == "" || rpmPackage.Version == "" || rpmPackage.Release == "" {
		err := fmt.Errorf("incomplete RPM package identity (name='%s', version='%s', release='%s')",
			rpmPackage.Name, rpmPackage.Version, rpmPackage.Release)
		return spdxmanifest.Package{}, err
	}

	evr := rpmPackage.Version + "-" + rpmPackage.Release
	if rpmPackage.Epoch != nil {
		evr = strconv.Itoa(*rpmPackage.Epoch) + ":" + evr
	}

	purl := purl.RpmPackageURL(namespace, rpmPackage.Name, rpmPackage.Epoch, rpmPackage.Version, rpmPackage.Release,
		rpmPackage.Arch)

	return spdxmanifest.Package{
		ID:      fmt.Sprintf("%s-%s.%s", rpmPackage.Name, evr, rpmPackage.Arch),
		Name:    rpmPackage.Name,
		Version: evr,
		Vendor:  strings.TrimSpace(rpmPackage.Vendor),
		Purl:    purl,
	}, nil
}

func rpmRemovePackageManagerTools(imageChroot *safechroot.Chroot, pmHandler rpmPackageManagerHandler,
	toolsChroot *safechroot.Chroot, packageManagementPackages []string,
) error {
	err := rpmEnsurePackagesRemoved(imageChroot, pmHandler, toolsChroot, packageManagementPackages,
		true /*removeProtectedPackages*/)
	if err != nil {
		return err
	}

	return nil
}

func rpmEnsurePackagesRemoved(imageChroot *safechroot.Chroot, pmHandler rpmPackageManagerHandler,
	toolsChroot *safechroot.Chroot, packages []string, removeProtectedPackages bool,
) error {
	packagesToRemove := []string(nil)
	for _, packageName := range packages {
		installed, err := pmHandler.isPackageInstalled(imageChroot, toolsChroot, packageName)
		if err != nil {
			return err
		}

		if installed {
			packagesToRemove = append(packagesToRemove, packageName)
		}
	}

	if len(packagesToRemove) <= 0 {
		// Nothing to do.
		return nil
	}

	args := getRpmRemoveArgs(pmHandler, toolsChroot, packagesToRemove, removeProtectedPackages)

	err := pmHandler.executeCommand(args, imageChroot, toolsChroot)
	if err != nil {
		return fmt.Errorf("%w (%v):\n%w", ErrPackageRemove, packagesToRemove, err)
	}

	return nil
}

func getRpmRootArgs(toolsChroot *safechroot.Chroot) []string {
	if toolsChroot == nil {
		return nil
	}

	// Run rpm from inside the tools chroot against the image bind-mounted at /_imageroot — needed when
	// imageChroot has no in-image rpm.
	return []string{"--root", "/" + toolsRootImageDir}
}

func getRpmInstallRootArgs(pmHandler rpmPackageManagerHandler, toolsChroot *safechroot.Chroot) []string {
	if toolsChroot == nil {
		return nil
	}

	return []string{
		"--releasever=" + pmHandler.getReleaseVersion(),
		"--installroot=/" + toolsRootImageDir,
	}
}

func getRpmRemoveArgs(pmHandler rpmPackageManagerHandler, toolsChroot *safechroot.Chroot, packages []string,
	removeProtectedPackages bool,
) []string {
	args := []string{"--assumeyes", "--disablerepo", "*"}
	if removeProtectedPackages {
		args = append(args, "--setopt=protected_packages=")
	}

	args = append(args, "remove")
	args = append(args, packages...)

	args = append(getRpmInstallRootArgs(pmHandler, toolsChroot), args...)

	return args
}
