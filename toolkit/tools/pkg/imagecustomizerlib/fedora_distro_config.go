// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/targetos"
)

// fedoraDistroConfig implements distroHandler for Fedora
type fedoraDistroConfig struct {
	version        string
	packageManager rpmPackageManagerHandler
}

func newFedoraDistroConfig(version string, packageManagerType PackageManagerType) *fedoraDistroConfig {
	var pm rpmPackageManagerHandler
	switch packageManagerType {
	case packageManagerDNF:
		pm = newDnfPackageManager(version)
	default:
		panic("unsupported package manager type for Fedora: " + string(packageManagerType))
	}

	return &fedoraDistroConfig{
		version:        version,
		packageManager: pm,
	}
}

func (d *fedoraDistroConfig) getDistroName() DistroName                   { return distroNameFedora }
func (d *fedoraDistroConfig) getPackageManager() rpmPackageManagerHandler { return d.packageManager }

func (d *fedoraDistroConfig) GetTargetOs() targetos.TargetOs {
	switch d.version {
	case "42":
		return targetos.TargetOsFedora42
	default:
		panic("unsupported Fedora version: " + d.version)
	}
}

// managePackages handles the complete package management workflow for Fedora
func (d *fedoraDistroConfig) managePackages(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.OS,
	imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	rpmsSources []string, useBaseImageRpmRepos bool, snapshotTime string,
) error {
	return managePackagesRpm(ctx, buildDir, baseConfigPath, config, imageChroot, toolsChroot, rpmsSources, useBaseImageRpmRepos, snapshotTime, d)
}
