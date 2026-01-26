// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
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

// DistroHandler represents the interface for distribution-specific configuration
type DistroHandler interface {
	GetTargetOs() targetos.TargetOs

	// Package management operations
	ManagePackages(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.OS,
		imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, rpmsSources []string, useBaseImageRpmRepos bool,
		snapshotTime imagecustomizerapi.PackageSnapshotTime) error

	IsPackageInstalled(imageChroot safechroot.ChrootInterface, packageName string) bool

	// Get all installed packages from the chroot
	GetAllPackagesFromChroot(imageChroot safechroot.ChrootInterface) ([]OsPackage, error)

	// Detect the bootloader type installed in the image
	DetectBootloaderType(imageChroot safechroot.ChrootInterface) (BootloaderType, error)

	// Get the path to the grub configuration file
	GetGrubConfigFilePath(imageChroot safechroot.ChrootInterface) string
}

// NewDistroHandlerFromTargetOs creates a distro handler directly from TargetOs
func NewDistroHandlerFromTargetOs(targetOs targetos.TargetOs) DistroHandler {
	switch targetOs {
	case targetos.TargetOsFedora42:
		return newFedoraDistroHandler("42")
	case targetos.TargetOsAzureLinux2:
		return newAzureLinuxDistroHandler("2.0")
	case targetos.TargetOsAzureLinux3:
		return newAzureLinuxDistroHandler("3.0")
	case targetos.TargetOsUbuntu2204:
		return newUbuntuDistroHandler("22.04")
	case targetos.TargetOsUbuntu2404:
		return newUbuntuDistroHandler("24.04")
	default:
		panic("unsupported target OS: " + string(targetOs))
	}
}

// NewDistroHandler creates the appropriate distro handler with version support (legacy)
func NewDistroHandler(distroName string, version string) DistroHandler {
	switch distroName {
	case string(distroNameFedora):
		return newFedoraDistroHandler(version)
	case string(distroNameAzureLinux):
		return newAzureLinuxDistroHandler(version)
	default:
		panic("unsupported distro name: " + distroName)
	}
}

// NewDistroHandlerFromChroot creates a distro handler by detecting the OS from the chroot
func NewDistroHandlerFromChroot(imageChroot safechroot.ChrootInterface) (DistroHandler, error) {
	targetOs, err := targetos.GetInstalledTargetOs(imageChroot.RootDir())
	if err != nil {
		return nil, err
	}
	return NewDistroHandlerFromTargetOs(targetOs), nil
}
