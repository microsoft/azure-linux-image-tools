// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
	"slices"
)

type ImageFormatType string

const (
	ImageFormatTypeNone           ImageFormatType = ""
	ImageFormatTypeVhd            ImageFormatType = "vhd"
	ImageFormatVhdTypeFixed       ImageFormatType = "vhd-fixed"
	ImageFormatTypeVhdx           ImageFormatType = "vhdx"
	ImageFormatTypeQcow2          ImageFormatType = "qcow2"
	ImageFormatTypeRaw            ImageFormatType = "raw"
	ImageFormatTypeIso            ImageFormatType = "iso"
	ImageFormatTypePxeDir         ImageFormatType = "pxe-dir"
	ImageFormatTypePxeTar         ImageFormatType = "pxe-tar"
	ImageFormatTypeCosi           ImageFormatType = "cosi"
	ImageFormatTypeBareMetalImage ImageFormatType = "baremetal-image"
)

// supportedImageFormatTypes is a list of all non-empty image format types
// defined above.
var supportedImageFormatTypes = []string{
	string(ImageFormatTypeVhd),
	string(ImageFormatVhdTypeFixed),
	string(ImageFormatTypeVhdx),
	string(ImageFormatTypeQcow2),
	string(ImageFormatTypeRaw),
	string(ImageFormatTypeIso),
	string(ImageFormatTypePxeDir),
	string(ImageFormatTypePxeTar),
	string(ImageFormatTypeCosi),
	string(ImageFormatTypeBareMetalImage),
}

var supportedImageFormatTypesImageCreator = []string{
	string(ImageFormatTypeVhd),
	string(ImageFormatVhdTypeFixed),
	string(ImageFormatTypeVhdx),
	string(ImageFormatTypeQcow2),
	string(ImageFormatTypeRaw),
}

func (ft ImageFormatType) IsValid() error {
	if ft != ImageFormatTypeNone && !slices.Contains(SupportedImageFormatTypes(), string(ft)) {
		return fmt.Errorf("invalid image format type (%s)", ft)
	}

	return nil
}

// SupportedImageFormatTypes returns all valid image format types.
func SupportedImageFormatTypes() []string {
	return supportedImageFormatTypes
}

func SupportedImageFormatTypesImageCreator() []string {
	return supportedImageFormatTypesImageCreator
}
