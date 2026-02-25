// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import "github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"

// debPackageManagerHandler represents the interface for DEB-based package managers (APT).
type debPackageManagerHandler interface {
	// getPackageManagerBinary returns the package manager binary name (e.g.  "apt-get").
	getPackageManagerBinary() string

	// getEnvironmentVariables returns the environment variables required for non-interactive operations.
	getEnvironmentVariables() []string

	isPackageInstalled(imageChroot safechroot.ChrootInterface, packageName string) bool
}
