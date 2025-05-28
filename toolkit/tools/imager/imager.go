// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Tool to create and install images

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
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
	app      = kingpin.New("imager", "Tool to create and install images.")
	buildDir = app.Flag("build-dir", "Directory to store temporary files while building.").ExistingDir()
	// imageFile = app.Flag("image-file", "Path of the base Azure Linux image which the customization will be applied to.").Required().ExistingFile()
	// outputImageFile  = app.Flag("output-image-file", "Path to write the customized image to.").Required().String()
	configFile = exe.InputFlag(app, "Path to the image config file.")
	localRepo  = app.Flag("local-repo", "Path to local RPM repo").ExistingDir()
	tdnfTar    = app.Flag("tools-file", "Path to tdnf worker tarball").ExistingFile()
	repoFile   = app.Flag("rpm-source", "Full path to local.repo.").ExistingFile()
	// assets           = app.Flag("assets", "Path to assets directory.").ExistingDir()
	baseDirPath = app.Flag("base-dir", "Base directory for relative file paths from the config. Defaults to config's directory.").ExistingDir()
	outputDir   = app.Flag("output-dir", "Path to directory to place final image.").ExistingDir()
	// take a new flaf fedora which is a boolean
	fedora = app.Flag("fedora", "Use fedora as the base image.").Bool()
	// imgContentFile   = app.Flag("output-image-contents", "File that stores list of packages used to compose the image.").String()
	// liveInstallFlag  = app.Flag("live-install", "Enable to perform a live install to the disk specified in config file.").Bool()
	emitProgress     = app.Flag("emit-progress", "Write progress updates to stdout, such as percent complete and current action.").Bool()
	timestampFile    = app.Flag("timestamp-file", "File that stores timestamps for this program.").String()
	buildNumber      = app.Flag("build-number", "Build number to be used in the image.").String()
	repoSnapshotTime = app.Flag("repo-snapshot-time", "Optional: Snapshot time to be added to the image tdnf.conf").String()
	logFlags         = exe.SetupLogFlags(app)
	profFlags        = exe.SetupProfileFlags(app)
)

func main() {
	const defaultSystemConfig = 0

	app.Version(exe.ToolkitVersion)
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

	if *fedora {
		logger.Log.Infof("Using fedora as the base image")
		imagecustomizerlib.Change_var_for_dnf()

	}

	var config imagecustomizerapi.Config
	err = imagecustomizerapi.UnmarshalYamlFile(*configFile, &config)
	if err != nil {
		logger.Log.Errorf("Failed to load configuration file (%s):\n%v", *configFile, err)
		return
	}

	baseConfigPath, _ := filepath.Split(*configFile)

	outputImageFile := "raw"

	useBaseImageRpmRepos := true

	var localRepoList []string
	if localRepo != nil && *localRepo != "" {
		localRepoList = append(localRepoList, *localRepo)
	}
	if repoFile != nil && *repoFile != "" {
		localRepoList = append(localRepoList, *repoFile)
	}
	logger.Log.Infof("LocalRepo: %s", localRepoList)

	err = imagecustomizerlib.ValidateConfig(baseConfigPath, &config, localRepoList, outputImageFile, useBaseImageRpmRepos)
	if err != nil {
		logger.Log.Errorf("Failed to validate configuration file (%s):\n%v", *configFile,
			err)
		return
	}

	// Create image customizer parameters

	buildDirAbs, err := filepath.Abs(*buildDir)
	if err != nil {
		logger.Log.Errorf("Failed to get absolute path for build directory (%s):\n%v", *buildDir, err)
		return
	}
	newBuildImageFile := filepath.Join(buildDirAbs, defaultTempDiskName)
	disks := config.Storage.Disks
	diskConfig := disks[0]
	installOSFunc := func(imageChroot *safechroot.Chroot) error {
		return nil
	}

	partIdToPartUuid, diskDevPath, err := imagecustomizerlib.CreateNewImage(targetos.TargetOsAzureLinux3, newBuildImageFile, diskConfig, config.Storage.FileSystems,
		*buildDir, setupRoot, installOSFunc)
	if err != nil {
		logger.Log.Errorf("Failed to create new image:\n%v", err)
		return
	}

	fmt.Printf("partIdToPartUuid: %v\n", partIdToPartUuid)

	// Create a uuid for the image
	imageUuid, imageUuidStr, err := imagecustomizerlib.CreateUuid()
	if err != nil {

		logger.Log.Errorf("Failed to create image uuid:\n%v", err)
		return
	}
	fmt.Printf("imageUuid: %v\n %v", imageUuid, imageUuidStr)

	// Customize the raw image file.
	fmt.Println("Customizing the image")

	partUuidToFstabEntry, osRelease, err := imagecustomizerlib.CustomizeImageHelperImager(buildDirAbs, baseConfigPath, &config, newBuildImageFile, localRepoList,
		false, false, imageUuidStr, diskDevPath, *tdnfTar)
	if err != nil {
		logger.Log.Errorf("Failed to customize image:\n%v", err)
		return
	}
	fmt.Printf("partUuidToFstabEntry: %v\n", partUuidToFstabEntry)
	fmt.Printf("osRelease: %v\n", osRelease)
}

const (
	localRepoMountPoint  = "/mnt/cdrom/RPMS"
	repoFileMountPoint   = "/etc/yum.repos.d"
	setupRoot            = "/setuproot"
	installRoot          = "/installroot"
	rootID               = "rootfs"
	defaultDiskIndex     = 0
	defaultTempDiskName  = "disk.raw"
	existingChrootDir    = false
	leaveChrootOnDisk    = false
	grub2Package         = "grub2"
	distroReleasePackage = "azurelinux-release"
)

var (
	isLoopDevice           bool
	isOfflineInstall       bool
	diskDevPath            string
	encryptedRoot          diskutils.EncryptedRootDevice
	partIDToDevPathMap     map[string]string
	partIDToFsTypeMap      map[string]string
	mountPointToOverlayMap map[string]*installutils.Overlay
	extraMountPoints       []*safechroot.MountPoint
	extraDirectories       []string
)
