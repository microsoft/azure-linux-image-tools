// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package main

import (
	"log"
	"os"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/exe"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/timestamp"
	"github.com/microsoft/azurelinux/toolkit/tools/pkg/imagecustomizerlib"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	app = kingpin.New("imagecustomizer", "Customizes a pre-built Azure Linux image")

	buildDir                    = app.Flag("build-dir", "Directory to run build out of.").Required().String()
	imageFile                   = app.Flag("image-file", "Path of the base Azure Linux image which the customization will be applied to.").Required().String()
	outputImageFile             = app.Flag("output-image-file", "Path to write the customized image to.").String()
	outputImageFormat           = app.Flag("output-image-format", "Format of output image. Supported: vhd, vhdx, qcow2, raw, iso, cosi.").Enum("vhd", "vhd-fixed", "vhdx", "qcow2", "raw", "iso", "cosi")
	outputSplitPartitionsFormat = app.Flag("output-split-partitions-format", "Format of partition files. Supported: raw, raw-zst").Enum("raw", "raw-zst")
	configFile                  = app.Flag("config-file", "Path of the image customization config file.").Required().String()
	rpmSources                  = app.Flag("rpm-source", "Path to a RPM repo config file or a directory containing RPMs.").Strings()
	disableBaseImageRpmRepos    = app.Flag("disable-base-image-rpm-repos", "Disable the base image's RPM repos as an RPM source").Bool()
	enableShrinkFilesystems     = app.Flag("shrink-filesystems", "Enable shrinking of filesystems to minimum size. Supports ext2, ext3, ext4 filesystem types.").Bool()
	outputPXEArtifactsDir       = app.Flag("output-pxe-artifacts-dir", "Create a directory with customized image PXE booting artifacts. '--output-image-format' must be set to 'iso'.").String()
	logFlags                    = exe.SetupLogFlags(app)
	timestampFile               = app.Flag("timestamp-file", "File that stores timestamps for this program.").String()
)

func main() {
	var err error

	app.Version(imagecustomizerlib.ToolVersion)
	kingpin.MustParse(app.Parse(os.Args[1:]))
	if *outputSplitPartitionsFormat == "" && *outputImageFormat == "" {
		kingpin.Fatalf("Either --output-image-format or --output-split-partitions-format must be specified.")
	}

	logger.InitBestEffort(logFlags)

	if *enableShrinkFilesystems && *outputSplitPartitionsFormat == "" {
		logger.Log.Fatalf("--output-split-partitions-format must be specified to use --shrink-filesystems.")
	}

	if *enableShrinkFilesystems && *outputImageFormat != "" {
		logger.Log.Fatalf("--output-image-format cannot be used with --shrink-filesystems enabled.")
	}

	if *timestampFile != "" {
		timestamp.BeginTiming("imagecustomizer", *timestampFile)
		defer timestamp.CompleteTiming()
	}

	err = customizeImage()
	if err != nil {
		log.Fatalf("image customization failed:\n%v", err)
	}
}

func customizeImage() error {
	var err error

	err = imagecustomizerlib.CustomizeImageWithConfigFile(*buildDir, *configFile, *imageFile,
		*rpmSources, *outputImageFile, *outputImageFormat, *outputSplitPartitionsFormat, *outputPXEArtifactsDir,
		!*disableBaseImageRpmRepos, *enableShrinkFilesystems)
	if err != nil {
		return err
	}

	return nil
}
