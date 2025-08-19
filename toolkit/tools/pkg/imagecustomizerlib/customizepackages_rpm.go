package imagecustomizerlib

import (
	"context"
	"fmt"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
)

// azureLinuxDistroConfig implements distroHandler for Azure Linux
type azureLinuxDistroConfig struct {
	version string
}

func (d *azureLinuxDistroConfig) getPackageManagerBinary() string { return "tdnf" }
func (d *azureLinuxDistroConfig) getPackageType() PackageType     { return packageTypeRPM }
func (d *azureLinuxDistroConfig) getReleaseVersion() string {
	if d.version != "" {
		return d.version
	}
	return "3.0" // default version
}
func (d *azureLinuxDistroConfig) getConfigFile() string       { return customTdnfConfRelPath }
func (d *azureLinuxDistroConfig) getDistroName() DistroName   { return distroNameAzureLinux }
func (d *azureLinuxDistroConfig) getPackageSourceDir() string { return rpmsMountParentDirInChroot }

// fedoraDistroConfig implements distroHandler for Fedora
type fedoraDistroConfig struct {
	version string
}

func (d *fedoraDistroConfig) getPackageManagerBinary() string { return "dnf" }
func (d *fedoraDistroConfig) getPackageType() PackageType     { return packageTypeRPM }
func (d *fedoraDistroConfig) getReleaseVersion() string {
	if d.version != "" {
		return d.version
	}
	return "42" // default version
}
func (d *fedoraDistroConfig) getConfigFile() string       { return "etc/dnf/dnf.conf" }
func (d *fedoraDistroConfig) getDistroName() DistroName   { return distroNameFedora }
func (d *fedoraDistroConfig) getPackageSourceDir() string { return rpmsMountParentDirInChroot }

// managePackagesRpm provides a shared implementation for RPM-based package management
func managePackagesRpm(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.OS,
	imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	rpmsSources []string, useBaseImageRpmRepos bool, snapshotTime string, distroConfig distroHandler,
) error {
	var err error

	packageManagerChroot := imageChroot
	if toolsChroot != nil {
		packageManagerChroot = toolsChroot
	}

	// Setup distribution-specific configuration if needed
	if distroConfig.getDistroName() == distroNameAzureLinux {
		// Setup Azure Linux specific TDNF configuration with snapshot
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
		err = refreshPackageMetadata(ctx, imageChroot, toolsChroot, distroConfig)
		if err != nil {
			return err
		}

		// Prefill cache for Fedora
		if distroConfig.getDistroName() == distroNameFedora {
			err = prefillDnfCache(ctx, config.Packages.Install, config.Packages.Update, imageChroot, toolsChroot, distroConfig)
			if err != nil {
				return err
			}
		}
	}

	// Execute package operations
	err = removePackages(ctx, config.Packages.Remove, imageChroot, toolsChroot, distroConfig)
	if err != nil {
		return err
	}

	if config.Packages.UpdateExistingPackages {
		err = updateAllPackages(ctx, imageChroot, toolsChroot, distroConfig)
		if err != nil {
			return err
		}
	}

	err = installOrUpdatePackages(ctx, "install", config.Packages.Install, imageChroot, toolsChroot, distroConfig)
	if err != nil {
		return err
	}

	err = installOrUpdatePackages(ctx, "update", config.Packages.Update, imageChroot, toolsChroot, distroConfig)
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
		err = cleanCache(imageChroot, toolsChroot, distroConfig)
		if err != nil {
			return err
		}
	}

	return nil
}

// managePackages handles the complete package management workflow for Azure Linux
func (d *azureLinuxDistroConfig) managePackages(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.OS,
	imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	rpmsSources []string, useBaseImageRpmRepos bool, snapshotTime string,
) error {
	return managePackagesRpm(ctx, buildDir, baseConfigPath, config, imageChroot, toolsChroot, rpmsSources, useBaseImageRpmRepos, snapshotTime, d)
}

// managePackages handles the complete package management workflow for Fedora
func (d *fedoraDistroConfig) managePackages(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.OS,
	imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	rpmsSources []string, useBaseImageRpmRepos bool, snapshotTime string,
) error {
	return managePackagesRpm(ctx, buildDir, baseConfigPath, config, imageChroot, toolsChroot, rpmsSources, useBaseImageRpmRepos, snapshotTime, d)
}

// prefillDnfCache downloads packages to DNF cache for offline installation
func prefillDnfCache(ctx context.Context, installPackages []string, updatePackages []string, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, distroConfig distroHandler) error {
	allPackages := append(installPackages, updatePackages...)
	if len(allPackages) == 0 {
		return nil
	}

	logger.Log.Infof("Pre-filling DNF cache for packages: %v", allPackages)

	args := []string{"-v", "install", "--assumeyes", "--downloadonly"}

	repoDir := distroConfig.getPackageSourceDir()
	if repoDir != "" {
		args = append(args, "--setopt=reposdir="+repoDir)
	}

	// Enable keepcache to ensure packages are stored in cache
	args = append(args, "--setopt=keepcache=1")

	if toolsChroot != nil {
		args = append([]string{"--releasever=" + distroConfig.getReleaseVersion(), "--installroot=/" + toolsRootImageDir}, args...)
	}

	args = append(args, allPackages...)

	err := executePackageManagerCommand(args, imageChroot, toolsChroot, distroConfig)
	if err != nil {
		return fmt.Errorf("failed to prefill DNF cache for packages (%v):\n%w", allPackages, err)
	}
	return nil
}
