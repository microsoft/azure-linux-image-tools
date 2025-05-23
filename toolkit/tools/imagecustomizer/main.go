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

type CustomizeCmd struct {
	BuildDir                 string   `name:"build-dir" help:"Directory to run build out of." required:""`
	InputImageFile           string   `name:"image-file" help:"Path of the base Azure Linux image which the customization will be applied to."`
	OutputImageFile          string   `name:"output-image-file" help:"Path to write the customized image to."`
	OutputImageFormat        string   `name:"output-image-format" placeholder:"(vhd|vhd-fixed|vhdx|qcow2|raw|iso|cosi)" help:"Format of output image." enum:"${imageformat}" default:""`
	ConfigFile               string   `name:"config-file" help:"Path of the image customization config file." required:""`
	RpmSources               []string `name:"rpm-source" help:"Path to a RPM repo config file or a directory containing RPMs."`
	DisableBaseImageRpmRepos bool     `name:"disable-base-image-rpm-repos" help:"Disable the base image's RPM repos as an RPM source."`
	OutputPXEArtifactsDir    string   `name:"output-pxe-artifacts-dir" help:"Create a directory with customized image PXE booting artifacts. '--output-image-format' must be set to 'iso'."`
	PackageSnapshotTime      string   `name:"package-snapshot-time" help:"Only packages published before this snapshot time will be available during customization. Supports 'YYYY-MM-DD' or full RFC3339 timestamp (e.g., 2024-05-20T23:59:59Z)."`
}

type InjectFilesCmd struct {
	BuildDir          string `name:"build-dir" help:"Directory to run build out of." required:""`
	ConfigFile        string `name:"config-file" help:"Path to the inject-files.yaml config file." required:""`
	InputImageFile    string `name:"image-file" help:"Path of the base image to inject files into." required:""`
	OutputImageFile   string `name:"output-image-file" help:"Path to write the injected image to."`
	OutputImageFormat string `name:"output-image-format" placeholder:"(vhd|vhd-fixed|vhdx|qcow2|raw|iso|cosi)" help:"Format of output image." enum:"${imageformat}" default:""`
}

type RootCmd struct {
	Customize     CustomizeCmd     `name:"customize" cmd:"" default:"withargs" help:"Customizes a pre-built Azure Linux image."`
	InjectFiles   InjectFilesCmd   `name:"inject-files" cmd:"" help:"Injects files into a partition based on an inject-files.yaml file."`
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
	case "customize":
		err := customizeImage(cli.Customize)
		if err != nil {
			log.Fatalf("image customization failed:\n%v", err)
		}

	case "inject-files":
		err := injectFiles(cli.InjectFiles)
		if err != nil {
			log.Fatalf("inject-files failed:\n%v", err)
		}

	default:
		panic(parseContext.Command())
	}
}

func customizeImage(cmd CustomizeCmd) error {
	err := imagecustomizerlib.CustomizeImageWithConfigFile(cmd.BuildDir, cmd.ConfigFile, cmd.InputImageFile,
		cmd.RpmSources, cmd.OutputImageFile, cmd.OutputImageFormat, cmd.OutputPXEArtifactsDir,
		!cmd.DisableBaseImageRpmRepos, cmd.PackageSnapshotTime)
	if err != nil {
		return err
	}

	return nil
}

func injectFiles(cmd InjectFilesCmd) error {
	err := imagecustomizerlib.InjectFilesWithConfigFile(cmd.BuildDir, cmd.ConfigFile, cmd.InputImageFile,
		cmd.OutputImageFile, cmd.OutputImageFormat)
	if err != nil {
		return err
	}

	return nil
}
