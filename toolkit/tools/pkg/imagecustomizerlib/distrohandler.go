// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/targetos"
)

// PackageManagerType represents the type of package manager
type PackageManagerType string

const (
	packageManagerTDNF PackageManagerType = "tdnf"
	packageManagerDNF  PackageManagerType = "dnf"
)

// PackageType represents the type of package format
type PackageType string

// DistroName represents the distribution name
type DistroName string

const (
	distroNameAzureLinux DistroName = "azurelinux"
	distroNameFedora     DistroName = "fedora"
)

// distroHandler represents the interface for distribution-specific configuration
type distroHandler interface {
	GetTargetOs() targetos.TargetOs

	// Package management operations
	managePackages(
		ctx context.Context,
		buildDir string,
		baseConfigPath string,
		config *imagecustomizerapi.OS,
		imageChroot *safechroot.Chroot,
		toolsChroot *safechroot.Chroot,
		rpmsSources []string,
		useBaseImageRpmRepos bool,
		snapshotTime string) error
}

// NewDistroHandlerFromTargetOs creates a distro handler directly from TargetOs
func NewDistroHandlerFromTargetOs(targetOs targetos.TargetOs) distroHandler {
	switch targetOs {
	case targetos.TargetOsFedora42:
		return newFedoraDistroHandler("42")
	case targetos.TargetOsAzureLinux2:
		return newAzureLinuxDistroHandler("2.0")
	case targetos.TargetOsAzureLinux3:
		return newAzureLinuxDistroHandler("3.0")
	default:
		panic("unsupported target OS: " + string(targetOs))
	}
}

// NewDistroHandlerFromImageConnection detects the OS from the image and creates the appropriate handler
func NewDistroHandlerFromImageConnection(imageConnection *imageconnection.ImageConnection) (distroHandler, error) {
	targetOs, err := targetos.GetInstalledTargetOs(imageConnection.Chroot().RootDir())
	if err != nil {
		return nil, err
	}

	return NewDistroHandlerFromTargetOs(targetOs), nil
}

// NewDistroHandler creates the appropriate distro handler with version support (legacy)
func NewDistroHandler(distroName string, version string) distroHandler {
	switch distroName {
	case string(distroNameFedora):
		return newFedoraDistroHandler(version)
	case string(distroNameAzureLinux):
		return newAzureLinuxDistroHandler(version)
	default:
		panic("unsupported distro name: " + distroName)
	}
}
