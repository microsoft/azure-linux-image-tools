// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
)

// ubuntuDistroHandler implements distroHandler for Ubuntu
type ubuntuDistroHandler struct {
	version string
}

func newUbuntuDistroHandler(version string) *ubuntuDistroHandler {
	return &ubuntuDistroHandler{
		version: version,
	}
}

func (d *ubuntuDistroHandler) GetTargetOs() targetos.TargetOs {
	switch d.version {
	case "22.04":
		return targetos.TargetOsUbuntu2204
	case "24.04":
		return targetos.TargetOsUbuntu2404
	default:
		panic("unsupported Ubuntu version: " + d.version)
	}
}

// managePackages handles the complete package management workflow for Ubuntu
func (d *ubuntuDistroHandler) managePackages(ctx context.Context, buildDir string, baseConfigPath string,
	config *imagecustomizerapi.OS, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	rpmsSources []string, useBaseImageRpmRepos bool, snapshotTime imagecustomizerapi.PackageSnapshotTime,
) error {
	if config != nil && (len(config.Packages.Install) > 0 || len(config.Packages.Remove) > 0 || len(config.Packages.Update) > 0) {
		return fmt.Errorf("package management customizations are not yet supported for Ubuntu")
	}

	return nil
}

// isPackageInstalled checks if a package is installed using dpkg
func (d *ubuntuDistroHandler) isPackageInstalled(imageChroot safechroot.ChrootInterface, packageName string) bool {
	return isPackageInstalledDpkg(imageChroot, packageName)
}
