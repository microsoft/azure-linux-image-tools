// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Tool to create and install images

package main

import (
	"log"
	"maps"

	"github.com/alecthomas/kong"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/exekong"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/ptrutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/timestamp"
	"github.com/microsoft/azurelinux/toolkit/tools/pkg/imagecreatorlib"
)

type ImageCreatorCmd struct {
	BuildDir      string   `name:"build-dir" help:"Directory to run build out of." required:""`
	ConfigFile    string   `name:"config-file" help:"Path of the image customization config file." required:""`
	RpmSources    []string `name:"rpm-source" help:"Path to a RPM repo config file or a directory containing RPMs."`
	ToolsTar      string   `name:"tools-file" help:"Path to tdnf worker tarball"`
	TimeStampFile string   `name:"timestamp-file" help:"File that stores timestamps for this program."`
	exekong.LogFlags
}

func main() {
	cli := &ImageCreatorCmd{}

	vars := kong.Vars{
		"version": imagecreatorlib.ToolVersion,
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

	if cli.TimeStampFile != "" {
		timestamp.BeginTiming("imagecreator", cli.TimeStampFile)
		defer timestamp.CompleteTiming()
	}
	err := imagecreatorlib.CreateImageWithConfigFile(cli.BuildDir, cli.ConfigFile, cli.RpmSources, cli.ToolsTar)
	if err != nil {
		log.Fatalf("image creation failed:\n%v", err)
	}
}
