// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Tool to create and install images

package main

import (
	"log"
	"maps"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/exekong"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/ptrutils"
	"github.com/microsoft/azurelinux/toolkit/tools/pkg/imagecreatorlib"
	"github.com/microsoft/azurelinux/toolkit/tools/pkg/imagecustomizerlib"
)

type ImageCreatorCmd struct {
	BuildDir          string   `name:"build-dir" help:"Directory to run build out of." required:""`
	ConfigFile        string   `name:"config-file" help:"Path of the image creator config file." required:""`
	RpmSources        []string `name:"rpm-source" help:"Path to a RPM repo config file or a directory containing RPMs." required:""`
	ToolsTar          string   `name:"tools-file" help:"Path to tdnf worker tarball" required:""`
	OutputImageFile   string   `name:"output-image-file" help:"Path to write the customized image to."`
	OutputImageFormat string   `name:"output-image-format" placeholder:"(vhd|vhd-fixed|vhdx|qcow2|raw)" help:"Format of output image." enum:"${imageformat}" default:""`
	exekong.LogFlags
}

func main() {
	cli := &ImageCreatorCmd{}

	vars := kong.Vars{
		"imageformat": strings.Join(imagecustomizerapi.SupportedImageFormatTypesImageCreator(), ",") + ",",
		"version":     imagecustomizerlib.ToolVersion,
	}
	maps.Copy(vars, exekong.KongVars)

	_ = kong.Parse(cli,
		vars,
		kong.HelpOptions{
			Compact:   true,
			FlagsLast: true,
		},
		kong.UsageOnError())

	logger.InitBestEffort(ptrutils.PtrTo(cli.LogFlags.AsLoggerFlags()))

	err := imagecreatorlib.CreateImageWithConfigFile(cli.BuildDir, cli.ConfigFile, cli.RpmSources, cli.ToolsTar, cli.OutputImageFile, cli.OutputImageFormat)
	if err != nil {
		log.Fatalf("image creation failed:\n%v", err)
	}
}
