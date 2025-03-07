// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
	"slices"
	"strings"
)

type ImageFormat string

const (
	ImageFormatVhd      ImageFormat = "vhd"
	ImageFormatVhdFixed ImageFormat = "vhd-fixed"
	ImageFormatVhdx     ImageFormat = "vhdx"
	ImageFormatQcow2    ImageFormat = "qcow2"
	ImageFormatRaw      ImageFormat = "raw"
	ImageFormatIso      ImageFormat = "iso"
	ImageFormatCosi     ImageFormat = "cosi"
)

// supportedImageFormats is a list of all image formats defined above.
var supportedImageFormats = []string{
	string(ImageFormatVhd),
	string(ImageFormatVhdFixed),
	string(ImageFormatVhdx),
	string(ImageFormatQcow2),
	string(ImageFormatRaw),
	string(ImageFormatIso),
	string(ImageFormatCosi),
}

func (f *ImageFormat) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var val string
	if err := unmarshal(&val); err != nil {
		return err
	}

	if slices.Contains(SupportedImageFormats(), val) {
		*f = ImageFormat(val)
		return nil
	}

	return fmt.Errorf("failed to parse image format: %s. Supported formats: %s",
		val, strings.Join(SupportedImageFormats(), ", "))
}

// SupportedImageFormats returns all valid image formats.
func SupportedImageFormats() []string {
	return supportedImageFormats
}
