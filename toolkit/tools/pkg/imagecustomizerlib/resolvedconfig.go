// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"path/filepath"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/randomization"
)

type ResolvedConfig struct {
	// Configurations
	BaseConfigPath        string
	Config                *imagecustomizerapi.Config
	Options               ImageCustomizerOptions
	CustomizeOSPartitions bool

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

func (c *ResolvedConfig) InputFileExt() string {
	fileExt := strings.TrimLeft(filepath.Ext(c.InputImageFile), ".")
	return fileExt
}

func (c *ResolvedConfig) InputIsIso() bool {
	imageFileExt := c.InputFileExt()
	inputIsIso := imageFileExt == string(imagecustomizerapi.ImageFormatTypeIso)
	return inputIsIso
}

func (c *ResolvedConfig) OutputIsIso() bool {
	return c.OutputImageFormat == imagecustomizerapi.ImageFormatTypeIso
}

func (c *ResolvedConfig) OutputIsPxe() bool {
	return c.OutputImageFormat == imagecustomizerapi.ImageFormatTypePxeDir ||
		c.OutputImageFormat == imagecustomizerapi.ImageFormatTypePxeTar
}
