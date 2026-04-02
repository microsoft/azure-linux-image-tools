// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package main

import (
	"context"
	"log"

	"github.com/alecthomas/kong"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/exekong"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/timestamp"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/pkg/osmodifierlib"
)

type RootCmd struct {
	ConfigFile    string `name:"config-file" help:"Path of the os modification config file."`
	TimeStampFile string `name:"timestamp-file" help:"File that stores timestamps for this program."`
	UpdateGrub    bool   `name:"update-grub" help:"Update default GRUB."`
	exekong.LogFlags
}

func main() {
	ctx := context.Background()

	var err error

	cli := &RootCmd{}
	_ = kong.Parse(cli,
		kong.Name("osmodifier"),
		kong.Description("Used to modify os"),
		exekong.KongVars,
		kong.HelpOptions{
			Compact:   true,
			FlagsLast: true,
		},
		kong.UsageOnError())

	logger.InitBestEffort(cli.LogFlags.AsLoggerFlags())

	timestamp.BeginTiming("osmodifier", cli.TimeStampFile)
	defer timestamp.CompleteTiming()

	// Check if the updateGrub flag is set
	if cli.UpdateGrub {
		err := osmodifierlib.ModifyDefaultGrub()
		if err != nil {
			log.Fatalf("update grub failed: %v", err)
		}
	}

	if cli.ConfigFile != "" {
		err = modifyImage(ctx, cli.ConfigFile)
		if err != nil {
			log.Fatalf("OS modification failed: %v", err)
		}
	}
}

func modifyImage(ctx context.Context, configFile string) error {
	err := osmodifierlib.ModifyOSWithConfigFile(ctx, configFile)
	if err != nil {
		return err
	}

	return nil
}
