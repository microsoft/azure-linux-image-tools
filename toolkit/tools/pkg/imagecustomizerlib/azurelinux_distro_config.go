// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/targetos"
)

// azureLinuxDistroConfig implements distroHandler for Azure Linux
type azureLinuxDistroConfig struct {
	version        string
	packageManager rpmPackageManagerHandler
}

func newAzureLinuxDistroConfig(version string, packageManagerType PackageManagerType) *azureLinuxDistroConfig {
	var pm rpmPackageManagerHandler
	switch packageManagerType {
	case packageManagerDNF:
		pm = newDnfPackageManager(version)
	case packageManagerTDNF:
		pm = newTdnfPackageManager(version)
	default:
		panic("unsupported package manager type for Azure Linux: " + string(packageManagerType))
	}

	return &azureLinuxDistroConfig{
		version:        version,
		packageManager: pm,
	}
}

func (d *azureLinuxDistroConfig) getDistroName() DistroName { return distroNameAzureLinux }
func (d *azureLinuxDistroConfig) getPackageManager() rpmPackageManagerHandler {
	return d.packageManager
}

func (d *azureLinuxDistroConfig) GetTargetOs() targetos.TargetOs {
	switch d.version {
	case "2.0":
		return targetos.TargetOsAzureLinux2
	case "3.0":
		return targetos.TargetOsAzureLinux3
	default:
		panic("unsupported Azure Linux version: " + d.version)
	}
}

// managePackages handles the complete package management workflow for Azure Linux
func (d *azureLinuxDistroConfig) managePackages(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.OS,
	imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	rpmsSources []string, useBaseImageRpmRepos bool, snapshotTime string,
) error {
	return managePackagesRpm(ctx, buildDir, baseConfigPath, config, imageChroot, toolsChroot, rpmsSources, useBaseImageRpmRepos, snapshotTime, d)
}
