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
	"github.com/microsoft/azurelinux/toolkit/tools/pkg/profile"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	app           = kingpin.New("imagecustomizer", "Customizes a pre-built Azure Linux image")
	logFlags      = exe.SetupLogFlags(app)
	profFlags     = exe.SetupProfileFlags(app)
	timestampFile = app.Flag("timestamp-file", "File that stores timestamps for this program.").String()

	runCmd                      = app.Command("run", "Customize an image").Default()
	buildDir                    = runCmd.Flag("build-dir", "Directory to run build out of.").Required().String()
	imageFile                   = runCmd.Flag("image-file", "Path of the base Azure Linux image which the customization will be applied to.").Required().String()
	outputImageFile             = runCmd.Flag("output-image-file", "Path to write the customized image to.").Required().String()
	outputImageFormat           = runCmd.Flag("output-image-format", "Format of output image. Supported: vhd, vhdx, qcow2, raw, iso.").Enum("vhd", "vhd-fixed", "vhdx", "qcow2", "raw", "iso")
	outputSplitPartitionsFormat = runCmd.Flag("output-split-partitions-format", "Format of partition files. Supported: raw, raw-zst").Enum("raw", "raw-zst")
	configFile                  = runCmd.Flag("config-file", "Path of the image customization config file.").Required().String()
	rpmSources                  = runCmd.Flag("rpm-source", "Path to a RPM repo config file or a directory containing RPMs.").Strings()
	disableBaseImageRpmRepos    = runCmd.Flag("disable-base-image-rpm-repos", "Disable the base image's RPM repos as an RPM source").Bool()
	enableShrinkFilesystems     = runCmd.Flag("shrink-filesystems", "Enable shrinking of filesystems to minimum size. Supports ext2, ext3, ext4 filesystem types.").Bool()
	outputPXEArtifactsDir       = runCmd.Flag("output-pxe-artifacts-dir", "Create a directory with customized image PXE booting artifacts. '--output-image-format' must be set to 'iso'.").String()

	validateCmd        = app.Command("validate-config", "Validates a config file")
	validateConfigFile = validateCmd.Flag("config-file", "Path of the image customization config file.").Required().String()
)

func main() {
	var err error

	app.Version(imagecustomizerlib.ToolVersion)
	command := kingpin.MustParse(app.Parse(os.Args[1:]))

	logger.InitBestEffort(logFlags)

	prof, err := profile.StartProfiling(profFlags)
	if err != nil {
		logger.Log.Warnf("Could not start profiling: %s", err)
	}
	defer prof.StopProfiler()

	timestamp.BeginTiming("imagecustomizer", *timestampFile)
	defer timestamp.CompleteTiming()

	switch command {
	case "run":
		err = customizeImage()
		if err != nil {
			log.Fatalf("image customization failed:\n%v", err)
		}

	case "validate-config":
		err = validateConfig()
		if err != nil {
			log.Fatalf("validate config failed:\n%v", err)
		}
	}
}

func customizeImage() error {
	if *outputSplitPartitionsFormat == "" && *outputImageFormat == "" {
		kingpin.Fatalf("Either --output-image-format or --output-split-partitions-format must be specified.")
	}

	if *enableShrinkFilesystems && *outputSplitPartitionsFormat == "" {
		logger.Log.Fatalf("--output-split-partitions-format must be specified to use --shrink-filesystems.")
	}

	if *enableShrinkFilesystems && *outputImageFormat != "" {
		logger.Log.Fatalf("--output-image-format cannot be used with --shrink-filesystems enabled.")
	}

	err := imagecustomizerlib.CustomizeImageWithConfigFile(*buildDir, *configFile, *imageFile,
		*rpmSources, *outputImageFile, *outputImageFormat, *outputSplitPartitionsFormat, *outputPXEArtifactsDir,
		!*disableBaseImageRpmRepos, *enableShrinkFilesystems)
	if err != nil {
		return err
	}

	return nil
}

func validateConfig() error {
	err := imagecustomizerlib.ValidateConfigFile(*validateConfigFile)
	if err != nil {
		return err
	}

	logger.Log.Infof("Ok")

	return nil
}
