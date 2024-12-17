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
	app = kingpin.New("imagecustomizer", "Customizes a pre-built Azure Linux image")

	buildDir                    = app.Flag("build-dir", "Directory to run build out of.").Required().String()
	imageFile                   = app.Flag("image-file", "Path of the base Azure Linux image which the customization will be applied to.").Required().String()
	outputImageFile             = app.Flag("output-image-file", "Path to write the customized image to.").Required().String()
	outputImageFormat           = app.Flag("output-image-format", "Format of output image. Supported: vhd, vhdx, qcow2, raw, iso.").Enum("vhd", "vhd-fixed", "vhdx", "qcow2", "raw", "iso")
	outputSplitPartitionsFormat = app.Flag("output-split-partitions-format", "Format of partition files. Supported: raw, raw-zst").Enum("raw", "raw-zst")
	configFile                  = app.Flag("config-file", "Path of the image customization config file.").Required().String()
	rpmSources                  = app.Flag("rpm-source", "Path to a RPM repo config file or a directory containing RPMs.").Strings()
	disableBaseImageRpmRepos    = app.Flag("disable-base-image-rpm-repos", "Disable the base image's RPM repos as an RPM source").Bool()
	enableShrinkFilesystems     = app.Flag("shrink-filesystems", "Enable shrinking of filesystems to minimum size. Supports ext2, ext3, ext4 filesystem types.").Bool()
	requireSignedRootfsRootHash = app.Flag("require-signed-rootfs-root-hash", "Requires that the verity root hash of the rootfs is signed.").Bool()
	requireSignedRootHashes     = app.Flag("require-signed-root-hashes", "Requires that all root hashes are signed.").Bool()
	outputPXEArtifactsDir       = app.Flag("output-pxe-artifacts-dir", "Create a directory with customized image PXE booting artifacts. '--output-image-format' must be set to 'iso'.").String()
	outputVerityHashes          = app.Flag("output-verity-hashes", "Save the root hash value of each verity target device in a text file.").Bool()
	outputVerityHashesDir       = app.Flag("output-verity-hashes-dir", "The directory where the verity root hash files will be saved to.").String()
	inputSignedVerityHashes     = app.Flag("input-signed-verity-hashes-files", "A list of one or more signed verity root hash files.").Strings()
	logFlags                    = exe.SetupLogFlags(app)
	profFlags                   = exe.SetupProfileFlags(app)
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

	prof, err := profile.StartProfiling(profFlags)
	if err != nil {
		logger.Log.Warnf("Could not start profiling: %s", err)
	}
	defer prof.StopProfiler()

	timestamp.BeginTiming("imagecustomizer", *timestampFile)
	defer timestamp.CompleteTiming()

	err = customizeImage()
	if err != nil {
		log.Fatalf("image customization failed:\n%v", err)
	}
}

func customizeImage() error {
	var err error

	err = imagecustomizerlib.CustomizeImageWithConfigFile(*buildDir, *configFile, *imageFile,
		*rpmSources, *outputImageFile, *outputImageFormat, *outputSplitPartitionsFormat, *outputPXEArtifactsDir,
		!*disableBaseImageRpmRepos, *requireSignedRootfsRootHash, *requireSignedRootHashes, *outputVerityHashes,
		*outputVerityHashesDir, *inputSignedVerityHashes, *enableShrinkFilesystems)
	if err != nil {
		return err
	}

	return nil
}
