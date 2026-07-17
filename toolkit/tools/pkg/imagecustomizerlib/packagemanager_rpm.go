// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/packagemanifestapi"
)

// rpmPackageManagerHandler represents the interface for RPM-based package managers (TDNF, DNF)
type rpmPackageManagerHandler interface {
	// Package manager configuration
	getReleaseVersion() string

	executeCommand(args []string, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	) ([]packagemanifestapi.Package, []packagemanifestapi.Package, error)

	// Package manager specific cache options for install/update operations
	getCacheOnlyOptions() []string

	configureSnapshotTime(packageManagerChroot *safechroot.Chroot,
		snapshotTime imagecustomizerapi.PackageSnapshotTime,
	) (func() error, error)

	// isPackageInstalled reports whether packageName is installed. When toolsChroot is non-nil, the query is issued
	// from inside toolsChroot against installroot=/_imageroot (the bind-mounted image root), so it works on images
	// that ship no in-image package manager.
	isPackageInstalled(imageChroot safechroot.ChrootInterface, toolsChroot *safechroot.Chroot, packageName string) (bool, error)

	// getPackageInformation queries the package database for packageName and returns its parsed version,
	// release, and distro fields.
	getPackageInformation(imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, packageName string) (*PackageVersionInformation, error)

	importGpgKeys(imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, chrootGpgKeys []string,
		uriGpgKeys []string,
	) error
}
