// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package main

import (
	"context"
	"fmt"
	"log"
	"maps"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/exekong"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/ptrutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/telemetry"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/timestamp"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/pkg/imagecustomizerlib"
)

type CustomizeCmd struct {
	BuildDir                 string   `name:"build-dir" help:"Directory to run build out of." required:""`
	InputImageFile           string   `name:"image-file" help:"Path of the base Azure Linux image which the customization will be applied to."`
	OutputImageFile          string   `name:"output-image-file" aliases:"output-path" help:"Path to write the customized image artifacts to."`
	OutputImageFormat        string   `name:"output-image-format" placeholder:"(vhd|vhd-fixed|vhdx|qcow2|raw|iso|pxe-dir|pxe-tar|cosi)" help:"Format of output image." enum:"${imageformat}" default:""`
	ConfigFile               string   `name:"config-file" help:"Path of the image customization config file." required:""`
	RpmSources               []string `name:"rpm-source" help:"Path to a RPM repo config file or a directory containing RPMs."`
	DisableBaseImageRpmRepos bool     `name:"disable-base-image-rpm-repos" help:"Disable the base image's RPM repos as an RPM source."`
	PackageSnapshotTime      string   `name:"package-snapshot-time" help:"Only packages published before this snapshot time will be available during customization. Supports 'YYYY-MM-DD' or full RFC3339 timestamp (e.g., 2024-05-20T23:59:59Z)."`
}

type InjectFilesCmd struct {
	BuildDir          string `name:"build-dir" help:"Directory to run build out of." required:""`
	ConfigFile        string `name:"config-file" help:"Path to the inject-files.yaml config file." required:""`
	InputImageFile    string `name:"image-file" help:"Path of the base image to inject files into." required:""`
	OutputImageFile   string `name:"output-image-file" aliases:"output-path" help:"Path to write the injected image to."`
	OutputImageFormat string `name:"output-image-format" placeholder:"(vhd|vhd-fixed|vhdx|qcow2|raw|iso|pxe-dir|pxe-tar|cosi)" help:"Format of output image." enum:"${imageformat}" default:""`
}

type RootCmd struct {
	Customize        CustomizeCmd     `name:"customize" cmd:"" default:"withargs" help:"Customizes a pre-built Azure Linux image."`
	InjectFiles      InjectFilesCmd   `name:"inject-files" cmd:"" help:"Injects files into a partition based on an inject-files.yaml file."`
	Version          kong.VersionFlag `name:"version" help:"Print version information and quit"`
	TimeStampFile    string           `name:"timestamp-file" help:"File that stores timestamps for this program."`
	DisableTelemetry bool             `name:"disable-telemetry" help:"Disable telemetry collection of the tool."`
	exekong.LogFlags
}

func main() {
	ctx := context.Background()

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

	err := runCommand(ctx, parseContext.Command(), cli)
	if err != nil {
		log.Fatalf("%v", err)
	}
}

func runCommand(ctx context.Context, command string, cli *RootCmd) error {
	// initialize OpenTelemetry tracer
	err := telemetry.InitTelemetry(cli.DisableTelemetry, imagecustomizerlib.ToolVersion)
	if err != nil {
		logger.Log.Warnf("Failed to initialize telemetry setup: %v", err)
	}
	defer func() {
		if err := telemetry.ShutdownTelemetry(ctx); err != nil {
			logger.Log.Warnf("Failed to shutdown telemetry: %v", err)
		}
	}()

	if cli.TimeStampFile != "" {
		timestamp.BeginTiming("imagecustomizer", cli.TimeStampFile)
		defer timestamp.CompleteTiming()
	}

	switch command {
	case "customize":
		err = customizeImage(ctx, cli.Customize)
		if err != nil {
			return fmt.Errorf("image customization failed:\n%w", err)
		}

	case "inject-files":
		err = injectFiles(ctx, cli.InjectFiles)
		if err != nil {
			return fmt.Errorf("inject-files failed:\n%w", err)
		}

	default:
		panic(command)
	}

	return nil
}

func customizeImage(ctx context.Context, cmd CustomizeCmd) error {
	err := imagecustomizerlib.CustomizeImageWithConfigFileOptions(ctx, cmd.ConfigFile,
		imagecustomizerlib.ImageCustomizerOptions{
			BuildDir:             cmd.BuildDir,
			InputImageFile:       cmd.InputImageFile,
			RpmsSources:          cmd.RpmSources,
			OutputImageFile:      cmd.OutputImageFile,
			OutputImageFormat:    imagecustomizerapi.ImageFormatType(cmd.OutputImageFormat),
			UseBaseImageRpmRepos: !cmd.DisableBaseImageRpmRepos,
			PackageSnapshotTime:  imagecustomizerapi.PackageSnapshotTime(cmd.PackageSnapshotTime),
		})
	if err != nil {
		return err
	}

	return nil
}

func injectFiles(ctx context.Context, cmd InjectFilesCmd) error {
	err := imagecustomizerlib.InjectFilesWithConfigFile(ctx, cmd.BuildDir, cmd.ConfigFile, cmd.InputImageFile,
		cmd.OutputImageFile, cmd.OutputImageFormat)
	if err != nil {
		return err
	}

	return nil
}
