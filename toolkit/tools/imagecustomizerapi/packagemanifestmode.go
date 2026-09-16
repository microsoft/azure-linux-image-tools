// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type PackageManifestMode string

const (
	PackageManifestModeUnspecified PackageManifestMode = ""
	PackageManifestModeCreate      PackageManifestMode = "create"
	PackageManifestModePassthrough PackageManifestMode = "passthrough"
	PackageManifestModeNone        PackageManifestMode = "none"
)

func (mode PackageManifestMode) IsValid() error {
	switch mode {
	case PackageManifestModeCreate, PackageManifestModePassthrough, PackageManifestModeNone:
		return nil

	default:
		return fmt.Errorf("invalid package manifest mode value (%s): must be one of ['create', 'passthrough', 'none']",
			mode)
	}
}
