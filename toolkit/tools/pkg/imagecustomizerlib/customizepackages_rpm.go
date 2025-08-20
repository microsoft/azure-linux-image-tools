package imagecustomizerlib

import (
	"context"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
)

// Distribution Configurations

// azureLinuxDistroConfig implements distroHandler for Azure Linux
type azureLinuxDistroConfig struct {
	packageManager packageManagerHandler
}

func newAzureLinuxDistroConfig(version string, packageManagerType PackageManagerType) *azureLinuxDistroConfig {
	var pm packageManagerHandler
	switch packageManagerType {
	case packageManagerDNF:
		pm = newDnfPackageManager(version)
	case packageManagerTDNF:
		fallthrough
	default:
		pm = newTdnfPackageManager(version)
	}

	return &azureLinuxDistroConfig{
		packageManager: pm,
	}
}

func (d *azureLinuxDistroConfig) getDistroName() DistroName                { return distroNameAzureLinux }
func (d *azureLinuxDistroConfig) getPackageManager() packageManagerHandler { return d.packageManager }

// fedoraDistroConfig implements distroHandler for Fedora
type fedoraDistroConfig struct {
	packageManager packageManagerHandler
}

func newFedoraDistroConfig(version string, packageManagerType PackageManagerType) *fedoraDistroConfig {
	var pm packageManagerHandler
	switch packageManagerType {
	case packageManagerDNF:
		fallthrough
	default:
		pm = newDnfPackageManager(version)
	}

	return &fedoraDistroConfig{
		packageManager: pm,
	}
}

func (d *fedoraDistroConfig) getDistroName() DistroName                { return distroNameFedora }
func (d *fedoraDistroConfig) getPackageManager() packageManagerHandler { return d.packageManager }

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

	// Get package manager from distribution config
	pmHandler := distroConfig.getPackageManager().(rpmPackageManagerHandler)

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
		err = refreshPackageMetadata(ctx, imageChroot, toolsChroot, pmHandler)
		if err != nil {
			return err
		}

		// Prefill cache for DNF package managers
		if pmHandler.getPackageManagerBinary() == string(packageManagerDNF) {
			err = prefillDnfCache(ctx, config.Packages.Install, config.Packages.Update, imageChroot, toolsChroot, pmHandler)
			if err != nil {
				return err
			}
		}
	}

	// Execute package operations
	err = removePackages(ctx, config.Packages.Remove, imageChroot, toolsChroot, pmHandler)
	if err != nil {
		return err
	}

	if config.Packages.UpdateExistingPackages {
		err = updateAllPackages(ctx, imageChroot, toolsChroot, pmHandler)
		if err != nil {
			return err
		}
	}

	err = installOrUpdatePackages(ctx, "install", config.Packages.Install, imageChroot, toolsChroot, pmHandler)
	if err != nil {
		return err
	}

	err = installOrUpdatePackages(ctx, "update", config.Packages.Update, imageChroot, toolsChroot, pmHandler)
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
		err = cleanCache(imageChroot, toolsChroot, pmHandler)
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
