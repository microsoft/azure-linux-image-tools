// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Tool to create and install images

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/installutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/exe"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/targetos"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/timestamp"
	"github.com/microsoft/azurelinux/toolkit/tools/pkg/imagecustomizerlib"
	"github.com/microsoft/azurelinux/toolkit/tools/pkg/profile"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	app           = kingpin.New("imager", "Tool to create seed image.")
	buildDir      = app.Flag("build-dir", "Directory to store temporary files while building.").ExistingDir()
	configFile    = exe.InputFlag(app, "Path to the image config file.")
	rpmsSource    = app.Flag("rpm-source", "Path to RPM sources, such as a local directory or a repo file.").Strings()
	toolsTar      = app.Flag("tools-file", "Path to tdnf worker tarball").ExistingFile()
	emitProgress  = app.Flag("emit-progress", "Write progress updates to stdout, such as percent complete and current action.").Bool()
	timestampFile = app.Flag("timestamp-file", "File that stores timestamps for this program.").String()
	logFlags      = exe.SetupLogFlags(app)
	profFlags     = exe.SetupProfileFlags(app)
)

func main() {
	app.Version(exe.ToolkitVersion)

	// Print the toolkit version and distro name
	fmt.Printf("Azure Linux Toolkit Version: %s\n", exe.ToolkitVersion)
	fmt.Printf("Distro Name Abbreviation: %s\n", exe.DistroNameAbbreviation)
	fmt.Printf("Distro Major Version: %s\n", exe.DistroMajorVersion)
	// Print the build number if provided
	kingpin.MustParse(app.Parse(os.Args[1:]))
	logger.InitBestEffort(logFlags)

	prof, err := profile.StartProfiling(profFlags)
	if err != nil {
		logger.Log.Warnf("Could not start profiling: %s", err)
	}
	defer prof.StopProfiler()

	timestamp.BeginTiming("imager", *timestampFile)
	defer timestamp.CompleteTiming()

	if *emitProgress {
		installutils.EnableEmittingProgress()
	}

	var config imagecustomizerapi.Config
	err = imagecustomizerapi.UnmarshalYamlFile(*configFile, &config)
	if err != nil {
		logger.Log.Errorf("Failed to load configuration file (%s):\n%v", *configFile, err)
		return
	}

	baseConfigPath, _ := filepath.Split(*configFile)

	// Create image customizer parameters
	buildDirAbs, err := filepath.Abs(*buildDir)
	if err != nil {
		logger.Log.Errorf("Failed to get absolute path for build directory (%s):\n%v", *buildDir, err)
		return
	}
	outputImageFile := filepath.Join(buildDirAbs, defaultTempDiskName)
	useBaseImageRpmRepos := false

	// Delete the output image file if it exists
	err = os.RemoveAll(outputImageFile)
	if err != nil {
		logger.Log.Errorf("Failed to remove output image file (%s):\n%v", outputImageFile, err)
		return
	}

	// Create the output image file
	file, err := os.Create(outputImageFile)
	if err != nil {
		logger.Log.Errorf("Failed to create output image file (%s):\n%v", outputImageFile, err)
		return
	}
	defer file.Close()

	// TODO: Add validation for the config file wrt the imager config
	err = imagecustomizerlib.ValidateConfig(baseConfigPath, &config, outputImageFile, *rpmsSource, outputImageFile, outputImageFormat, useBaseImageRpmRepos, "")
	if err != nil {
		logger.Log.Errorf("Failed to validate configuration file (%s):\n%v", *configFile,
			err)
		return
	}

	disks := config.Storage.Disks
	diskConfig := disks[0]
	installOSFunc := func(imageChroot *safechroot.Chroot) error {
		return nil
	}

	// TODO: Get the target OS from the config or command line argument
	partIdToPartUuid, diskDevPath, err := imagecustomizerlib.CreateNewImage(targetos.TargetOsAzureLinux3, outputImageFile, diskConfig, config.Storage.FileSystems,
		*buildDir, setupRoot, installOSFunc)
	if err != nil {
		logger.Log.Errorf("Failed to create new image:\n%v", err)
		return
	}

	fmt.Printf("part id to part uuid map %v\n", partIdToPartUuid)

	// Create a uuid for the image
	imageUuid, imageUuidStr, err := imagecustomizerlib.CreateUuid()
	if err != nil {

		logger.Log.Errorf("Failed to create image uuid:\n%v", err)
		return
	}
	fmt.Printf("Created imageUuid: %v\n %v", imageUuid, imageUuidStr)
	fmt.Println("Customizing the image")

	// TODO: Add support for package snapshot time
	partUuidToFstabEntry, osRelease, err := imagecustomizerlib.CustomizeImageHelperImager(buildDirAbs, baseConfigPath, &config, outputImageFile, *rpmsSource,
		false, false, imageUuidStr, diskDevPath, "", *toolsTar)
	if err != nil {
		logger.Log.Errorf("Failed to customize image:\n%v", err)
		return
	}
	fmt.Printf("Part uuid to fstab entry: %v\n", partUuidToFstabEntry)
	fmt.Printf("osRelease: %v\n", osRelease)
}

const (
	setupRoot           = "/setuproot"
	defaultTempDiskName = "disk.raw"
	outputImageFormat   = "raw"
)
