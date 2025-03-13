// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

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
	"github.com/microsoft/azurelinux/toolkit/tools/internal/timestamp"
	"github.com/microsoft/azurelinux/toolkit/tools/pkg/imagecustomizerlib"
)

type BuildCmd struct {
	BuildDir                 string   `name:"build-dir" help:"Directory to run build out of." required:""`
	InputImageFile           string   `name:"image-file" help:"Path of the base Azure Linux image which the customization will be applied to."`
	OutputImageFile          string   `name:"output-image-file" help:"Path to write the customized image to."`
	OutputImageFormat        string   `name:"output-image-format" placeholder:"(vhd|vhd-fixed|vhdx|qcow2|raw|iso|cosi)" help:"Format of output image." enum:"${imageformat}" default:""`
	ConfigFile               string   `name:"config-file" help:"Path of the image customization config file." required:""`
	RpmSources               []string `name:"rpm-source" help:"Path to a RPM repo config file or a directory containing RPMs."`
	DisableBaseImageRpmRepos bool     `name:"disable-base-image-rpm-repos" help:"Disable the base image's RPM repos as an RPM source."`
	OutputPXEArtifactsDir    string   `name:"output-pxe-artifacts-dir" help:"Create a directory with customized image PXE booting artifacts. '--output-image-format' must be set to 'iso'."`
}

type RootCmd struct {
	Build         BuildCmd         `name:"build" cmd:"" default:"withargs" help:"Customizes a pre-built Azure Linux image."`
	Version       kong.VersionFlag `name:"version" help:"Print version information and quit"`
	TimeStampFile string           `name:"timestamp-file" help:"File that stores timestamps for this program."`
	exekong.LogFlags
}

func main() {
	cli := &RootCmd{}

	vars := kong.Vars{
		"imageformat": strings.Join(imagecustomizerapi.SupportedImageFormatTypes(), ",") + ",",
		"version":     imagecustomizerlib.ToolVersion,
	}
	maps.Copy(vars, exekong.KongVars)

	parseContext := kong.Parse(cli,
		vars,
		kong.HelpOptions{
			Compact:   true,
			FlagsLast: true,
		},
		kong.UsageOnError())

	logger.InitBestEffort(ptrutils.PtrTo(cli.LogFlags.AsLoggerFlags()))

	if cli.TimeStampFile != "" {
		timestamp.BeginTiming("imagecustomizer", cli.TimeStampFile)
		defer timestamp.CompleteTiming()
	}

	switch parseContext.Command() {
	case "build":
		err := customizeImage(cli.Build)
		if err != nil {
			log.Fatalf("image customization failed:\n%v", err)
		}

	default:
		panic(parseContext.Command())
	}
}

func customizeImage(buildCmd BuildCmd) error {
	err := imagecustomizerlib.CustomizeImageWithConfigFile(buildCmd.BuildDir, buildCmd.ConfigFile,
		buildCmd.InputImageFile, buildCmd.RpmSources, buildCmd.OutputImageFile, buildCmd.OutputImageFormat,
		buildCmd.OutputPXEArtifactsDir, !buildCmd.DisableBaseImageRpmRepos)
	if err != nil {
		return err
	}

	return nil
}
