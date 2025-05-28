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

// supportedInitramfsImageTypes is a list of all non-empty image format types
// defined above.
var supportedInitramfsImageTypes = []string{
	string(InitramfsImageTypeBootstrap),
	string(InitramfsImageTypeFullOS),
}

func (ft InitramfsImageType) IsValid() error {
	if ft != InitramfsImageTypeUnspecified && !slices.Contains(SupportedInitramfsImageTypes(), string(ft)) {
		return fmt.Errorf("invalid initramfs image type (%s)", ft)
	}

	return nil
}

// SupportedImageFormatTypes returns all valid image format types.
func SupportedInitramfsImageTypes() []string {
	return supportedInitramfsImageTypes
}
