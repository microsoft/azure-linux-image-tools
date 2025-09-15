// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Tool to create and install images

package main

import (
	"context"
	"log"
	"maps"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/exekong"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/ptrutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/pkg/imagecreatorlib"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/pkg/imagecustomizerlib"
)

type ImageCreatorCmd struct {
	BuildDir          string   `name:"build-dir" help:"Directory to run build out of." required:""`
	ConfigFile        string   `name:"config-file" help:"Path of the image creator config file." required:""`
	RpmSources        []string `name:"rpm-source" help:"Path to a RPM repo config file or a directory containing RPMs." required:""`
	ToolsTar          string   `name:"tools-file" help:"Path to tdnf worker tarball" required:""`
	OutputImageFile   string   `name:"output-image-file" help:"Path to write the customized image to."`
	OutputImageFormat string   `name:"output-image-format" placeholder:"(vhd|vhd-fixed|vhdx|qcow2|raw)" help:"Format of output image." enum:"${imageformat}" default:""`
	Distro            string   `name:"distro" help:"Target distribution for the image." enum:"azurelinux,fedora" default:"azurelinux"`
	DistroVersion     string   `name:"distro-version" help:"Target distribution version (e.g., 3.0 for Azure Linux, 42 for Fedora)." default:""`
	exekong.LogFlags
	PackageSnapshotTime string `name:"package-snapshot-time" help:"Only packages published before this snapshot time will be available during customization. Supports 'YYYY-MM-DD' or full RFC3339 timestamp (e.g., 2024-05-20T23:59:59Z)."`
}

func main() {
	ctx := context.Background()

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

	err := imagecreatorlib.CreateImageWithConfigFile(ctx, cli.BuildDir, cli.ConfigFile, cli.RpmSources,
		cli.ToolsTar, cli.OutputImageFile, cli.OutputImageFormat, cli.Distro, cli.DistroVersion,
		cli.PackageSnapshotTime)
	if err != nil {
		log.Fatalf("image creation failed:\n%v", err)
	}
}
