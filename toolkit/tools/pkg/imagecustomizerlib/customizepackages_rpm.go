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
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/purl"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/spdxmanifest"
	"github.com/sirupsen/logrus"
)

const (
	// The ARCH guard excludes gpg-pubkey entries, which are signing keys rather than installed packages.
	rpmQueryFormat = `%|ARCH?{%{NEVRA}\t%{VENDOR}\n}:{}|`

	// rpmTagNone is what rpm's query format prints for a tag the package does not carry.
	rpmTagNone = "(none)"

	rpmQueryColumns = 2
)

var (
	ErrInvalidName            = errors.New("invalid RPM package name")
	ErrInvalidVersion         = errors.New("invalid RPM package version")
	ErrInvalidRelease         = errors.New("invalid RPM package release")
	ErrInvalidArch            = errors.New("invalid RPM package architecture")
	ErrNevraArchSeparator     = errors.New("missing architecture separator in RPM NEVRA")
	ErrNevraReleaseSeparator  = errors.New("missing release separator in RPM NEVRA")
	ErrNevraVersionSeparator  = errors.New("missing version separator in RPM NEVRA")
	ErrInvalidRpmQueryColumns = errors.New("expected '<nevra>\\t<vendor>'")
)

// RpmPackage is an installed RPM, identified by its name, epoch, version, release and architecture.
// Vendor is not identifying information, but is included in the RPM query output.
type RpmPackage struct {
	Name    string
	Epoch   string
	Version string
	Release string
	Arch    string
	Vendor  string
}

// Evr renders the package's "[epoch:]version-release".
func (packageInfo RpmPackage) Evr() string {
	if packageInfo.Epoch == "" {
		return packageInfo.Version + "-" + packageInfo.Release
	}

	return packageInfo.Epoch + ":" + packageInfo.Version + "-" + packageInfo.Release
}

// Nevra renders the package's "name-[epoch:]version-release.arch".
func (packageInfo RpmPackage) Nevra() string {
	return packageInfo.Name + "-" + packageInfo.Evr() + "." + packageInfo.Arch
}

func (packageInfo RpmPackage) Nvra() string {
	return packageInfo.Name + "-" + packageInfo.Version + "-" + packageInfo.Release + "." + packageInfo.Arch
}

// newRpmPackage builds an RpmPackage from its name, epoch, version, release, architecture, and vendor.
func newRpmPackage(name string, epoch string, version string, release string, arch string, vendor string) (RpmPackage, error) {
	if name == "" {
		return RpmPackage{}, fmt.Errorf("%w, expected a non-empty name (version='%s', release='%s')",
			ErrInvalidName, version, release)
	}

	if version == "" {
		return RpmPackage{}, fmt.Errorf("%w, expected a non-empty version (name='%s')", ErrInvalidVersion, name)
	}

	if release == "" {
		return RpmPackage{}, fmt.Errorf("%w, expected a non-empty release (name='%s')", ErrInvalidRelease, name)
	}

	if arch == "" {
		return RpmPackage{}, fmt.Errorf("%w, expected a non-empty architecture (name='%s')", ErrInvalidArch, name)
	}

	vendor = strings.TrimSpace(vendor)
	if vendor == rpmTagNone {
		vendor = ""
	}

	return RpmPackage{
		Name:    name,
		Epoch:   epoch,
		Version: version,
		Release: release,
		Arch:    arch,
		Vendor:  vendor,
	}, nil
}

// newRpmPackageFromNevra builds an RpmPackage from a "name-[epoch:]version-release.arch" NEVRA string.
func newRpmPackageFromNevra(nevra string, vendor string) (RpmPackage, error) {
	nevra = strings.TrimSpace(nevra)

	archIndex := strings.LastIndex(nevra, ".")
	if archIndex < 0 {
		return RpmPackage{}, fmt.Errorf("%w, expected 'name-[epoch:]version-release.arch' (nevra='%s')",
			ErrNevraArchSeparator, nevra)
	}

	nevr, arch := nevra[:archIndex], nevra[archIndex+1:]

	// The epoch, version, and release cannot contain '-', so the dashes separate them unambiguously.
	releaseIndex := strings.LastIndex(nevr, "-")
	if releaseIndex < 0 {
		return RpmPackage{}, fmt.Errorf("%w, expected 'name-[epoch:]version-release.arch' (nevra='%s')",
			ErrNevraReleaseSeparator, nevra)
	}

	versionIndex := strings.LastIndex(nevr[:releaseIndex], "-")
	if versionIndex < 0 {
		return RpmPackage{}, fmt.Errorf("%w, expected 'name-[epoch:]version-release.arch' (nevra='%s')",
			ErrNevraVersionSeparator, nevra)
	}

	name := nevr[:versionIndex]

	epoch := ""
	version := nevr[versionIndex+1 : releaseIndex]
	if colon := strings.LastIndex(version, ":"); colon >= 0 {
		epoch, version = version[:colon], version[colon+1:]
	}

	release := nevr[releaseIndex+1:]

	return newRpmPackage(name, epoch, version, release, arch, vendor)
}

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

	_, err := pmHandler.executeCommand(args, imageChroot, toolsChroot)
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

	_, err := pmHandler.executeCommand(args, imageChroot, toolsChroot)
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

	_, err := pmHandler.executeCommand(args, imageChroot, toolsChroot)
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

	_, err := pmHandler.executeCommand(args, imageChroot, toolsChroot)
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

	_, err = pmHandler.executeCommand(args, imageChroot, toolsChroot)
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

	_, err := pmHandler.executeCommand(args, imageChroot, toolsChroot)
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

func listInstalledPackagesRpm(imageChroot safechroot.ChrootInterface, toolsChroot *safechroot.Chroot,
	purlNamespace string,
) ([]spdxmanifest.Package, error) {
	args := []string{"-qa", "--queryformat", rpmQueryFormat}
	args = append(getRpmRootArgs(toolsChroot), args...)
	chroot := getRpmChroot(imageChroot, toolsChroot)

	// Query RPM directly because tdnf does not report the package vendor.
	out, _, err := shell.NewExecBuilder("rpm", args...).
		LogLevel(logrus.TraceLevel, logrus.DebugLevel).
		Chroot(chroot.ChrootDir()).
		ExecuteCaptureOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list the image's installed packages:\n%w", err)
	}

	packages, err := parseRpmPackageList(out)
	if err != nil {
		return nil, err
	}

	installedPackages := make([]spdxmanifest.Package, 0, len(packages))
	for _, packageInfo := range packages {
		installedPackages = append(installedPackages, newManifestPackageFromRpmPackage(packageInfo, purlNamespace))
	}
	return installedPackages, nil
}

// parseRpmPackageList reads the lines `rpm -qa --qf rpmQueryFormat` writes to stdout.
func parseRpmPackageList(output string) ([]RpmPackage, error) {
	packages := []RpmPackage(nil)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		columns := strings.Split(line, "\t")
		if len(columns) != rpmQueryColumns {
			return nil, fmt.Errorf("%w (line='%s')", ErrInvalidRpmQueryColumns, line)
		}

		packageInfo, err := newRpmPackageFromNevra(columns[0], columns[1])
		if err != nil {
			return nil, err
		}

		packages = append(packages, packageInfo)
	}

	return packages, nil
}

func newManifestPackageFromRpmPackage(rpmPackage RpmPackage, namespace string) spdxmanifest.Package {
	return spdxmanifest.Package{
		ID:      rpmPackage.Nevra(),
		Name:    rpmPackage.Name,
		Version: rpmPackage.Evr(),
		Vendor:  rpmPackage.Vendor,
		Purl: purl.RpmPackageURL(namespace, rpmPackage.Name, rpmPackage.Version, rpmPackage.Release,
			rpmPackage.Arch, rpmPackage.Epoch),
	}
}

func rpmRemovePackageManagerTools(imageChroot *safechroot.Chroot, pmHandler rpmPackageManagerHandler,
	toolsChroot *safechroot.Chroot, packageManagementPackages []string,
) ([]string, error) {
	packagesToRemove, err := rpmInstalledSubset(imageChroot, pmHandler, toolsChroot, packageManagementPackages)
	if err != nil {
		return nil, err
	}

	if len(packagesToRemove) <= 0 {
		// Nothing to do.
		return []string{}, nil
	}

	args := getRpmRemoveArgs(pmHandler, toolsChroot, packagesToRemove,
		true /* removeProtectedPackages */)

	removedPackageIds, err := pmHandler.executeCommand(args, imageChroot, toolsChroot)
	if err != nil {
		return nil, fmt.Errorf("%w (%v):\n%w", ErrPackageRemove, packagesToRemove, err)
	}

	return removedPackageIds, nil
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

func rpmInstalledSubset(imageChroot safechroot.ChrootInterface, pmHandler rpmPackageManagerHandler,
	toolsChroot *safechroot.Chroot, packages []string,
) ([]string, error) {
	installedPackages := []string(nil)
	for _, packageName := range packages {
		installed, err := pmHandler.isPackageInstalled(imageChroot, toolsChroot, packageName)
		if err != nil {
			return nil, err
		}

		if installed {
			installedPackages = append(installedPackages, packageName)
		}
	}

	return installedPackages, nil
}
