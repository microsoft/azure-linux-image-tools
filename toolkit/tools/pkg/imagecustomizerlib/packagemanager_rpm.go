// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

// rpmPackageManagerHandler represents the interface for RPM-based package managers (TDNF, DNF)
type rpmPackageManagerHandler interface {
	// Package manager configuration
	getPackageManagerBinary() string
	getReleaseVersion() string
	getConfigFile() string

	// Package manager specific output handling
	createOutputCallback() func(string)

	// Package manager specific cache options for install/update operations
	getCacheOnlyOptions() []string

	// Package manager specific snapshot time support
	supportsSnapshotTime() bool
}
