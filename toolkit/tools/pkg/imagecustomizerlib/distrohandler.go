// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
)

// PackageType represents the type of package manager
type PackageType string

const (
	packageTypeRPM PackageType = "rpm"
	packageTypeDeb PackageType = "deb"
)

// DistroName represents the distribution name
type DistroName string

const (
	distroNameAzureLinux DistroName = "azurelinux"
	distroNameFedora     DistroName = "fedora"
)

// distroHandler represents the interface for distribution-specific configuration
type distroHandler interface {
	// Package manager configuration
	getPackageManagerBinary() string
	getPackageType() PackageType
	getReleaseVersion() string
	getConfigFile() string

	// Distribution identification
	getDistroName() DistroName

	// Package source handling
	getPackageSourceDir() string

	// Package management operations
	managePackages(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.OS,
		imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
		rpmsSources []string, useBaseImageRpmRepos bool, snapshotTime string) error
}

// NewDistroHandler creates the appropriate distro handler with version support
func NewDistroHandler(distroName string, version string) distroHandler {
	switch distroName {
	case string(distroNameFedora):
		return &fedoraDistroConfig{version: version}
	case string(distroNameAzureLinux):
		fallthrough
	default:
		return &azureLinuxDistroConfig{version: version}
	}
}
