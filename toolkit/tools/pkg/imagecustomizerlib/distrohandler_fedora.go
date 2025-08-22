// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/targetos"
)

// fedoraDistroHandler implements distroHandler for Fedora
type fedoraDistroHandler struct {
	version        string
	packageManager rpmPackageManagerHandler
}

func newFedoraDistroHandler(version string, packageManagerType PackageManagerType) *fedoraDistroHandler {
	var pm rpmPackageManagerHandler
	switch packageManagerType {
	case packageManagerDNF:
		pm = newDnfPackageManager(version)
	default:
		panic("unsupported package manager type for Fedora: " + string(packageManagerType))
	}

	return &fedoraDistroHandler{
		version:        version,
		packageManager: pm,
	}
}

func (d *fedoraDistroHandler) getDistroName() DistroName { return distroNameFedora }

func (d *fedoraDistroHandler) getPackageManager() rpmPackageManagerHandler { return d.packageManager }

func (d *fedoraDistroHandler) GetTargetOs() targetos.TargetOs {
	switch d.version {
	case "42":
		return targetos.TargetOsFedora42
	default:
		panic("unsupported Fedora version: " + d.version)
	}
}

// managePackages handles the complete package management workflow for Fedora
func (d *fedoraDistroHandler) managePackages(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.OS,
	imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	rpmsSources []string, useBaseImageRpmRepos bool, snapshotTime string,
) error {
	return managePackagesRpm(ctx, buildDir, baseConfigPath, config, imageChroot, toolsChroot, rpmsSources, useBaseImageRpmRepos, snapshotTime, d)
}
