// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package main

import (
	"context"
	"log"
	"os"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/exe"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/timestamp"
	"github.com/microsoft/azurelinux/toolkit/tools/pkg/osmodifierlib"
	"github.com/microsoft/azurelinux/toolkit/tools/pkg/profile"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	app = kingpin.New("osmodifier", "Used to modify os")

	configFile    = app.Flag("config-file", "Path of the os modification config file.").String()
	logFlags      = exe.SetupLogFlags(app)
	profFlags     = exe.SetupProfileFlags(app)
	timestampFile = app.Flag("timestamp-file", "File that stores timestamps for this program.").String()
	updateGrub    = app.Flag("update-grub", "Update default GRUB.").Bool()
)

func main() {
	ctx := context.Background()

	var err error

	kingpin.MustParse(app.Parse(os.Args[1:]))

	logger.InitBestEffort(logFlags)

	prof, err := profile.StartProfiling(profFlags)
	if err != nil {
		logger.Log.Warnf("Could not start profiling: %s", err)
	}
	defer prof.StopProfiler()

	timestamp.BeginTiming("osmodifier", *timestampFile)
	defer timestamp.CompleteTiming()

	// Check if the updateGrub flag is set
	if *updateGrub {
		err := osmodifierlib.ModifyDefaultGrub()
		if err != nil {
			log.Fatalf("update grub failed: %v", err)
		}
	}

	if len(*configFile) > 0 {
		err = modifyImage(ctx)
		if err != nil {
			log.Fatalf("OS modification failed: %v", err)
		}
	}
}

func modifyImage(ctx context.Context) error {
	err := osmodifierlib.ModifyOSWithConfigFile(ctx, *configFile)
	if err != nil {
		return err
	}

	return nil
}
