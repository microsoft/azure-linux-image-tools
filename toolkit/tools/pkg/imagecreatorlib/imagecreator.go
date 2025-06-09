package imagecreatorlib

import (
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/randomization"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/targetos"
	"github.com/microsoft/azurelinux/toolkit/tools/pkg/imagecustomizerlib"
)

const (
	setupRoot         = "/setuproot"
	outputImageFormat = "raw"
)

func CreateImageWithConfigFile(buildDir string, configFile string,
	rpmsSources []string,
	toolsTar string,
	outputImageFile string,
) error {
	var config imagecustomizerapi.Config
	err := imagecustomizerapi.UnmarshalYamlFile(configFile, &config)
	if err != nil {
		return err
	}

	baseConfigPath, _ := filepath.Split(configFile)
	useBaseImageRpmRepos := false

	err = createNewImage(buildDir, baseConfigPath, config, outputImageFile, rpmsSources, outputImageFile, outputImageFormat, useBaseImageRpmRepos, toolsTar)
	if err != nil {
		return err
	}

	return nil
}

func createNewImage(buildDir string, baseConfigPath string, config imagecustomizerapi.Config, inputImageFile string,
	rpmsSources []string, outputImageFile string, outputImageFormat string,
	useBaseImageRpmRepos bool, toolsTar string,
) error {
	// TODO: Add validation for the config file wrt the imager config
	err := imagecustomizerlib.ValidateConfig(baseConfigPath, &config, inputImageFile, rpmsSources, outputImageFile, outputImageFormat, useBaseImageRpmRepos, "", true)
	if err != nil {
		return err
	}

	disks := config.Storage.Disks
	diskConfig := disks[0]
	installOSFunc := func(imageChroot *safechroot.Chroot) error {
		return nil
	}

	// TODO: Get the target OS from the config or command line argument
	partIdToPartUuid, err := imagecustomizerlib.CreateNewImage(targetos.TargetOsAzureLinux3, outputImageFile, diskConfig, config.Storage.FileSystems,
		buildDir, setupRoot, installOSFunc)
	if err != nil {
		return err
	}

	logger.Log.Debugf("part id to part uuid map %v\n", partIdToPartUuid)

	// Create a uuid for the image
	imageUuid, imageUuidStr, err := randomization.CreateUuid()
	if err != nil {
		return err
	}
	logger.Log.Debugf("Created imageUuid: %v\n %v", imageUuid, imageUuidStr)

	buildDirAbs, err := filepath.Abs(buildDir)
	if err != nil {
		return err
	}
	partUuidToFstabEntry, osRelease, err := imagecustomizerlib.CustomizeImageHelperImageCreator(buildDirAbs, baseConfigPath, &config, outputImageFile, rpmsSources,
		false, imageUuidStr, "", toolsTar)
	if err != nil {
		return err
	}
	logger.Log.Debugf("Part uuid to fstab entry: %v\n", partUuidToFstabEntry)
	logger.Log.Debugf("osRelease: %v\n", osRelease)

	return nil
}
