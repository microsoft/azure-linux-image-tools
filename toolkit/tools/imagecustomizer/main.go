// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/exe"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/timestamp"
	"github.com/microsoft/azurelinux/toolkit/tools/pkg/imagecustomizerlib"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	app = kingpin.New("imagecustomizer", "Customizes a pre-built Azure Linux image")

	buildDir                 = app.Flag("build-dir", "Directory to run build out of.").Required().String()
	inputImageFile           = app.Flag("image-file", "Path of the base Azure Linux image which the customization will be applied to.").String()
	outputImageFile          = app.Flag("output-image-file", "Path to write the customized image to.").String()
	outputImageFormat        = app.Flag("output-image-format", fmt.Sprintf("Format of output image. Supported: %s.", strings.Join(imagecustomizerapi.SupportedImageFormatTypes(), ", "))).Enum(imagecustomizerapi.SupportedImageFormatTypes()...)
	configFile               = app.Flag("config-file", "Path of the image customization config file.").Required().String()
	rpmSources               = app.Flag("rpm-source", "Path to a RPM repo config file or a directory containing RPMs.").Strings()
	disableBaseImageRpmRepos = app.Flag("disable-base-image-rpm-repos", "Disable the base image's RPM repos as an RPM source").Bool()
	outputPXEArtifactsDir    = app.Flag("output-pxe-artifacts-dir", "Create a directory with customized image PXE booting artifacts. '--output-image-format' must be set to 'iso'.").String()
	logFlags                 = exe.SetupLogFlags(app)
	timestampFile            = app.Flag("timestamp-file", "File that stores timestamps for this program.").String()
)

func main() {
	var err error

	app.Version(imagecustomizerlib.ToolVersion)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	logger.InitBestEffort(logFlags)

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
	err := imagecustomizerlib.CustomizeImageWithConfigFile(*buildDir, *configFile, *inputImageFile,
		*rpmSources, *outputImageFile, *outputImageFormat, *outputPXEArtifactsDir,
		!*disableBaseImageRpmRepos)
	if err != nil {
		return err
	}

	return nil
}
