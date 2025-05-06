// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
	"slices"
)

type InitramfsImageType string

const (
	InitramfsImageTypeUnspecified InitramfsImageType = ""
	InitramfsImageTypeBootstrap   InitramfsImageType = "bootstrap"
	InitramfsImageTypeFullOS      InitramfsImageType = "full-os"
)

// supportedInitramfsImageTypes is a list of all non-empty initramfs image types
// defined above.
var supportedInitramfsImageTypes = []string{
	string(InitramfsImageTypeBootstrap),
	string(InitramfsImageTypeFullOS),
}

func (it InitramfsImageType) IsValid() error {
	if it != InitramfsImageTypeUnspecified && !slices.Contains(SupportedInitramfsImageTypes(), string(it)) {
		return fmt.Errorf("invalid initramfs image type (%s)", it)
	}

	return nil
}

// SupportedInitramfsImageTypes returns all valid initramfs image types.
func SupportedInitramfsImageTypes() []string {
	return supportedInitramfsImageTypes
}
