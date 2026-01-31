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
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/pkg/imagecreatorlib"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/pkg/imagecustomizerlib"
)

type CreateCmd struct {
	BuildDir            string   `name:"build-dir" help:"Directory to run build out of." required:""`
	ConfigFile          string   `name:"config-file" help:"Path of the image creator config file." required:""`
	RpmSources          []string `name:"rpm-source" help:"Path to a RPM repo config file or a directory containing RPMs." required:""`
	ToolsTar            string   `name:"tools-file" help:"Path to tdnf/dnf worker tarball" required:""`
	OutputImageFile     string   `name:"output-image-file" aliases:"output-path" help:"Path to write the customized image to."`
	OutputImageFormat   string   `name:"output-image-format" placeholder:"(vhd|vhd-fixed|vhdx|qcow2|raw)" help:"Format of output image." enum:"${imageformatcreate}" default:""`
	Distro              string   `name:"distro" help:"Target distribution for the image." enum:"azurelinux,fedora" default:"azurelinux"`
	DistroVersion       string   `name:"distro-version" help:"Target distribution version (e.g., 3.0 for Azure Linux, 42 for Fedora)." default:""`
	PackageSnapshotTime string   `name:"package-snapshot-time" help:"Only packages published before this snapshot time will be available during customization. Supports 'YYYY-MM-DD' or full RFC3339 timestamp (e.g., 2024-05-20T23:59:59Z)."`
}

type CustomizeCmd struct {
	BuildDir                 string   `name:"build-dir" help:"Directory to run build out of." required:""`
	InputImageFile           string   `name:"image-file" help:"Path of the base Azure Linux image which the customization will be applied to."`
	InputImage               string   `name:"image" help:"The image which the customization will be applied to.\n Supported formats:\n - oci:URI"`
	OutputImageFile          string   `name:"output-image-file" aliases:"output-path" help:"Path to write the customized image artifacts to."`
	OutputImageFormat        string   `name:"output-image-format" placeholder:"(vhd|vhd-fixed|vhdx|qcow2|raw|iso|pxe-dir|pxe-tar|cosi|baremetal-image)" help:"Format of output image." enum:"${imageformat}" default:""`
	OutputSelinuxPolicyPath  string   `name:"output-selinux-policy-path" help:"Path to output directory for extracting SELinux policy files."`
	ConfigFile               string   `name:"config-file" help:"Path of the image customization config file." required:""`
	RpmSources               []string `name:"rpm-source" help:"Path to a RPM repo config file or a directory containing RPMs."`
	DisableBaseImageRpmRepos bool     `name:"disable-base-image-rpm-repos" help:"Disable the base image's RPM repos as an RPM source."`
	PackageSnapshotTime      string   `name:"package-snapshot-time" help:"Only packages published before this snapshot time will be available during customization. Supports 'YYYY-MM-DD' or full RFC3339 timestamp (e.g., 2024-05-20T23:59:59Z)."`
	ImageCacheDir            string   `name:"image-cache-dir" help:"The directory to use as the image download cache"`
	CosiCompressionLevel     *int     `name:"cosi-compression-level" help:"Zstd compression level for COSI output (1-22, default: 9)."`
}

type InjectFilesCmd struct {
	BuildDir             string `name:"build-dir" help:"Directory to run build out of." required:""`
	ConfigFile           string `name:"config-file" help:"Path to the inject-files.yaml config file." required:""`
	InputImageFile       string `name:"image-file" help:"Path of the base image to inject files into." required:""`
	OutputImageFile      string `name:"output-image-file" aliases:"output-path" help:"Path to write the injected image to."`
	OutputImageFormat    string `name:"output-image-format" placeholder:"(vhd|vhd-fixed|vhdx|qcow2|raw|iso|pxe-dir|pxe-tar|cosi|baremetal-image)" help:"Format of output image." enum:"${imageformat}" default:""`
	CosiCompressionLevel *int   `name:"cosi-compression-level" help:"Zstd compression level for COSI output (1-22, default: 9)."`
}

type RootCmd struct {
	Create           CreateCmd        `name:"create" cmd:"" help:"Creates a new Azure Linux image from scratch."`
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
		"imageformat":       strings.Join(imagecustomizerapi.SupportedImageFormatTypes(), ",") + ",",
		"imageformatcreate": strings.Join(imagecustomizerapi.SupportedImageFormatTypesImageCreator(), ",") + ",",
		"version":           imagecustomizerlib.ToolVersion,
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
	case "create":
		err = createImage(ctx, cli.Create)
		if err != nil {
			return fmt.Errorf("image creation failed:\n%w", err)
		}

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
			BuildDir:                cmd.BuildDir,
			InputImageFile:          cmd.InputImageFile,
			InputImage:              cmd.InputImage,
			RpmsSources:             cmd.RpmSources,
			OutputImageFile:         cmd.OutputImageFile,
			OutputImageFormat:       imagecustomizerapi.ImageFormatType(cmd.OutputImageFormat),
			OutputSelinuxPolicyPath: cmd.OutputSelinuxPolicyPath,
			UseBaseImageRpmRepos:    !cmd.DisableBaseImageRpmRepos,
			PackageSnapshotTime:     imagecustomizerapi.PackageSnapshotTime(cmd.PackageSnapshotTime),
			ImageCacheDir:           cmd.ImageCacheDir,
			CosiCompressionLevel:    cmd.CosiCompressionLevel,
		})
	if err != nil {
		return err
	}

	return nil
}

func injectFiles(ctx context.Context, cmd InjectFilesCmd) error {
	err := imagecustomizerlib.InjectFilesWithConfigFile(ctx, cmd.ConfigFile,
		imagecustomizerlib.InjectFilesOptions{
			BuildDir:             cmd.BuildDir,
			InputImageFile:       cmd.InputImageFile,
			OutputImageFile:      cmd.OutputImageFile,
			OutputImageFormat:    cmd.OutputImageFormat,
			CosiCompressionLevel: cmd.CosiCompressionLevel,
		})
	if err != nil {
		return err
	}

	return nil
}

func createImage(ctx context.Context, cmd CreateCmd) error {
	err := imagecreatorlib.CreateImageWithConfigFile(ctx, cmd.BuildDir, cmd.ConfigFile, cmd.RpmSources,
		cmd.ToolsTar, cmd.OutputImageFile, cmd.OutputImageFormat, cmd.Distro, cmd.DistroVersion,
		cmd.PackageSnapshotTime)
	if err != nil {
		return err
	}

	return nil
}
