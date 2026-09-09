// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
)

// rpmPackageManagerHandler represents the interface for RPM-based package managers (TDNF, DNF)
type rpmPackageManagerHandler interface {
	// Package manager configuration
	getReleaseVersion() string

	executeCommand(args []string, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot) error

	// Package manager specific cache options for install/update operations
	getCacheOnlyOptions() []string

	configureSnapshotTime(packageManagerChroot *safechroot.Chroot,
		snapshotTime imagecustomizerapi.PackageSnapshotTime,
	) (func() error, error)

	// isPackageInstalled reports whether packageName is installed. When toolsChroot is non-nil, the query is issued
	// from inside toolsChroot against installroot=/_imageroot (the bind-mounted image root), so it works on images
	// that ship no in-image package manager.
	isPackageInstalled(imageChroot safechroot.ChrootInterface, toolsChroot *safechroot.Chroot, packageName string) (bool, error)

	importGpgKeys(imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, chrootGpgKeys []string,
		uriGpgKeys []string,
	) error
}
