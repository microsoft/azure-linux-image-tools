// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"path/filepath"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/randomization"
)

type resolvedConfig struct {
	// Configurations
	BaseConfigPath string
	Config         *imagecustomizerapi.Config
	Options        ImageCustomizerOptions

	// UUID
	ImageUuid    [randomization.UuidSize]byte
	ImageUuidStr string

	// Build dirs
	BuildDirAbs string

	// Input image
	InputImageFile string

	// Output image
	OutputImageFile   string
	OutputImageFormat imagecustomizerapi.ImageFormatType

	// Packages and repos
	PackageSnapshotTime imagecustomizerapi.PackageSnapshotTime

	// Intermediate writeable image.
	RawImageFile string
}

func (c *resolvedConfig) InputFileExt() string {
	fileExt := strings.TrimLeft(filepath.Ext(c.InputImageFile), ".")
	return fileExt
}

func (c *resolvedConfig) InputIsIso() bool {
	imageFileExt := c.InputFileExt()
	inputIsIso := imageFileExt == string(imagecustomizerapi.ImageFormatTypeIso)
	return inputIsIso
}

func (c *resolvedConfig) CustomizeOSPartitions() bool {
	customizeOSPartitions := c.Config.CustomizePartitions() ||
		c.Config.OS != nil ||
		len(c.Config.Scripts.PostCustomization) > 0 ||
		len(c.Config.Scripts.FinalizeCustomization) > 0
	return customizeOSPartitions
}

func (c *resolvedConfig) OutputIsIso() bool {
	return c.OutputImageFormat == imagecustomizerapi.ImageFormatTypeIso
}

func (c *resolvedConfig) OutputIsPxe() bool {
	return c.OutputImageFormat == imagecustomizerapi.ImageFormatTypePxeDir ||
		c.OutputImageFormat == imagecustomizerapi.ImageFormatTypePxeTar
}
