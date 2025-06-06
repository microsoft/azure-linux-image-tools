package imagecreatorlib

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/randomization"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/targetos"
	"github.com/microsoft/azurelinux/toolkit/tools/pkg/imagecustomizerlib"
)

const (
	setupRoot           = "/setuproot"
	defaultTempDiskName = "disk.raw"
	outputImageFormat   = "raw"
)

// Version specifies the version of the Azure Linux Image Creator tool.
// The value of this string is inserted during compilation via a linker flag.
var ToolVersion = ""

func CreateImageWithConfigFile(buildDir string, configFile string,
	rpmsSources []string,
	toolsTar string,
) error {
	var config imagecustomizerapi.Config
	err := imagecustomizerapi.UnmarshalYamlFile(configFile, &config)
	if err != nil {
		return err
	}

	baseConfigPath, _ := filepath.Split(configFile)

	// Create image customizer parameters
	buildDirAbs, err := filepath.Abs(buildDir)
	if err != nil {
		return err
	}
	outputImageFile := filepath.Join(buildDirAbs, defaultTempDiskName)
	useBaseImageRpmRepos := false

	// Delete the output image file if it exists
	err = os.RemoveAll(outputImageFile)
	if err != nil {
		return err
	}

	// Create the output image file
	file, err := os.Create(outputImageFile)
	if err != nil {
		return err
	}
	defer file.Close()

	createImage(buildDir, baseConfigPath, config, outputImageFile, rpmsSources, outputImageFile, outputImageFormat, useBaseImageRpmRepos, toolsTar)
	if err != nil {
		return err
	}

	return nil
}

func createImage(buildDir string, baseConfigPath string, config imagecustomizerapi.Config, inputImageFile string,
	rpmsSources []string, outputImageFile string, outputImageFormat string,
	useBaseImageRpmRepos bool, toolsTar string,
) error {
	// TODO: Add validation for the config file wrt the imager config
	err := imagecustomizerlib.ValidateConfig(baseConfigPath, &config, inputImageFile, rpmsSources, outputImageFile, outputImageFormat, useBaseImageRpmRepos, "")
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

	fmt.Printf("part id to part uuid map %v\n", partIdToPartUuid)

	// Create a uuid for the image
	imageUuid, imageUuidStr, err := randomization.CreateUuid()
	if err != nil {
		return err
	}
	fmt.Printf("Created imageUuid: %v\n %v", imageUuid, imageUuidStr)
	fmt.Println("Customizing the image")

	buildDirAbs, err := filepath.Abs(buildDir)
	if err != nil {
		return err
	}
	partUuidToFstabEntry, osRelease, err := imagecustomizerlib.CustomizeImageHelperImageCreator(buildDirAbs, baseConfigPath, &config, outputImageFile, rpmsSources,
		false, imageUuidStr, "", toolsTar)
	if err != nil {
		return err
	}
	fmt.Printf("Part uuid to fstab entry: %v\n", partUuidToFstabEntry)
	fmt.Printf("osRelease: %v\n", osRelease)

	return nil
}
