// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

// DistroName represents the distribution name
type DistroName string

const (
	DistroNameDefault    DistroName = ""
	DistroNameAzureLinux DistroName = "azurelinux"
	DistroNameFedora     DistroName = "fedora"
	DistroNameUbuntu     DistroName = "ubuntu"
)

func (n DistroName) IsValid() error {
	switch n {
	case DistroNameDefault,
		DistroNameAzureLinux,
		DistroNameFedora,
		DistroNameUbuntu:
		// All good.
		return nil

	default:
		return fmt.Errorf("invalid distroName value (%s)", n)
	}
}
