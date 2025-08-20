// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
)

// PackageManagerType represents the type of package manager
type PackageManagerType string

const (
	packageManagerTDNF PackageManagerType = "tdnf"
	packageManagerDNF  PackageManagerType = "dnf"
)

// PackageType represents the type of package format
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

// packageManagerHandler represents the interface for package manager implementation details
type packageManagerHandler interface {
	// Package manager configuration
	getPackageManagerBinary() string
	getPackageType() PackageType
	getReleaseVersion() string
	getConfigFile() string
	getPackageSourceDir() string

	// Package manager specific output handling
	createOutputCallback() func(string)
}

// rpmPackageManagerHandler represents the interface for RPM-based package managers (TDNF, DNF)
type rpmPackageManagerHandler interface {
	packageManagerHandler

	// RPM-specific functionality is already covered by packageManagerHandler
	// This interface exists to enforce type safety for RPM-only operations
}

// distroHandler represents the interface for distribution-specific configuration
type distroHandler interface {
	// Distribution identification
	getDistroName() DistroName

	// Get the package manager for this distribution
	getPackageManager() packageManagerHandler

	// Package management operations
	managePackages(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.OS,
		imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
		rpmsSources []string, useBaseImageRpmRepos bool, snapshotTime string) error
}

// NewDistroHandler creates the appropriate distro handler with version support
func NewDistroHandler(distroName string, version string) distroHandler {
	switch distroName {
	case string(distroNameFedora):
		return newFedoraDistroConfig(version, packageManagerDNF)
	case string(distroNameAzureLinux):
		fallthrough
	default:
		return newAzureLinuxDistroConfig(version, packageManagerTDNF)
	}
}

// NewDistroHandlerWithPackageManager creates a distro handler with specific package manager
func NewDistroHandlerWithPackageManager(distroName string, version string, pmType PackageManagerType) distroHandler {
	switch distroName {
	case string(distroNameFedora):
		return newFedoraDistroConfig(version, pmType)
	case string(distroNameAzureLinux):
		fallthrough
	default:
		return newAzureLinuxDistroConfig(version, pmType)
	}
}
