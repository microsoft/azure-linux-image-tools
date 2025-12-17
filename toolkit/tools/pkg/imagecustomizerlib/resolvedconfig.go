// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"path/filepath"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/randomization"
)

// ResolvedConfig contains the final image configuration, including the merged CLI and config values.
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
	InputImage imagecustomizerapi.InputImage

	// Output artifacts
	OutputArtifacts *imagecustomizerapi.Artifacts

	// Output SELinux policy path
	OutputSelinuxPolicyPath string

	// Output image
	OutputImageFile   string
	OutputImageFormat imagecustomizerapi.ImageFormatType

	// Intermediate writeable image.
	RawImageFile string

	// Hostname
	Hostname string

	// SELinux
	SELinux imagecustomizerapi.SELinux

	// Bootloader
	BootLoader imagecustomizerapi.BootLoader

	// Kernel command line
	KernelCommandLine imagecustomizerapi.KernelCommandLine

	// UKI
	Uki *imagecustomizerapi.Uki

	// COSI compression level
	CosiCompressionLevel int

	// COSI compression long window size
	CosiCompressionLong int

	// Hierarchical config chain
	ConfigChain []*ConfigWithBasePath
}

func (c *ResolvedConfig) InputFileExt() string {
	fileExt := strings.TrimLeft(filepath.Ext(c.InputImage.Path), ".")
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
