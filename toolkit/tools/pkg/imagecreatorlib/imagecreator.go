package imagecreatorlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/randomization"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/targetos"
	"github.com/microsoft/azurelinux/toolkit/tools/pkg/imagecustomizerlib"
)

const (
	setupRoot = "/setuproot"
)

type ImageCreatorParameters struct {
	// build dirs
	buildDirAbs string

	// configurations
	configPath          string
	config              *imagecustomizerapi.Config
	rpmsSources         []string
	packageSnapshotTime string
	toolsTar            string

	// output image
	outputImageFormat imagecustomizerapi.ImageFormatType
	outputImageFile   string
	outputImageDir    string
	outputImageBase   string

	// raw image file
	rawImageFile string

	imageUuid    [randomization.UuidSize]byte
	imageUuidStr string

	partUuidToFstabEntry map[string]diskutils.FstabEntry
	osRelease            string
}

func CreateImageWithConfigFile(ctx context.Context, buildDir string, configFile string,
	rpmsSources []string,
	toolsTar string,
	outputImageFile string,
	outputImageFormat string,
	packageSnapshotTime string,
) error {
	var config imagecustomizerapi.Config
	err := imagecustomizerapi.UnmarshalYamlFile(configFile, &config)
	if err != nil {
		return fmt.Errorf("failed to unmarshal config file %s:\n%w", configFile, err)
	}

	baseConfigPath, _ := filepath.Split(configFile)

	absBaseConfigPath, err := filepath.Abs(baseConfigPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path of config file directory:\n%w", err)
	}

	err = createNewImage(ctx, buildDir, absBaseConfigPath, config, rpmsSources, outputImageFile, outputImageFormat, toolsTar, packageSnapshotTime)
	if err != nil {
		return err
	}

	return nil
}

func createNewImage(ctx context.Context, buildDir string, baseConfigPath string, config imagecustomizerapi.Config,
	rpmsSources []string, outputImageFile string, outputImageFormat string,
	toolsTar string, packageSnapshotTime string,
) error {
	if toolsTar == "" {
		return fmt.Errorf("tools tar file is required for image creation")
	}

	err := validateConfig(ctx, baseConfigPath, &config, rpmsSources,
		outputImageFile, outputImageFormat, packageSnapshotTime)
	if err != nil {
		return err
	}

	imageCreatorParameters, err := createImageCreatorParameters(buildDir,
		baseConfigPath, &config, rpmsSources,
		outputImageFormat, outputImageFile, packageSnapshotTime, toolsTar)
	if err != nil {
		return fmt.Errorf("invalid parameters:\n%w", err)
	}

	err = imagecustomizerlib.CheckEnvironmentVars()
	if err != nil {
		return err
	}

	imagecustomizerlib.LogVersionsOfToolDeps()

	// ensure build and output folders are created up front
	err = os.MkdirAll(imageCreatorParameters.buildDirAbs, os.ModePerm)
	if err != nil {
		return err
	}

	err = os.MkdirAll(imageCreatorParameters.outputImageDir, os.ModePerm)
	if err != nil {
		return err
	}

	disks := imageCreatorParameters.config.Storage.Disks
	diskConfig := disks[0]
	installOSFunc := func(imageChroot *safechroot.Chroot) error {
		return nil
	}

	logger.Log.Infof("Creating new image with parameters: %+v\n", imageCreatorParameters)

	// TODO: Get the target OS from the config or command line argument
	partIdToPartUuid, err := imagecustomizerlib.CreateNewImage(targetos.TargetOsAzureLinux3, imageCreatorParameters.rawImageFile, diskConfig, imageCreatorParameters.config.Storage.FileSystems,
		imageCreatorParameters.buildDirAbs, setupRoot, installOSFunc)
	if err != nil {
		return err
	}

	logger.Log.Debugf("Part id to part uuid map %v\n", partIdToPartUuid)
	logger.Log.Infof("Image UUID: %s", imageCreatorParameters.imageUuidStr)

	partUuidToFstabEntry, osRelease, err := imagecustomizerlib.CustomizeImageHelperImageCreator(ctx, imageCreatorParameters.buildDirAbs, imageCreatorParameters.configPath, imageCreatorParameters.config, imageCreatorParameters.rawImageFile, imageCreatorParameters.rpmsSources,
		false, imageCreatorParameters.imageUuidStr, imageCreatorParameters.packageSnapshotTime, imageCreatorParameters.toolsTar)
	if err != nil {
		return err
	}

	imageCreatorParameters.partUuidToFstabEntry = partUuidToFstabEntry
	imageCreatorParameters.osRelease = osRelease
	logger.Log.Debugf("Part uuid to fstab entry: %v\n", partUuidToFstabEntry)
	logger.Log.Debugf("OsRelease: %v\n", osRelease)

	logger.Log.Infof("Writing: %s", imageCreatorParameters.outputImageFile)

	err = imagecustomizerlib.ConvertImageFile(imageCreatorParameters.rawImageFile, imageCreatorParameters.outputImageFile, imageCreatorParameters.outputImageFormat)
	if err != nil {
		return err
	}
	logger.Log.Infof("Success!")

	return nil
}

func createImageCreatorParameters(buildDir string,
	configPath string, config *imagecustomizerapi.Config,
	rpmsSources []string,
	outputImageFormat string, outputImageFile string, packageSnapshotTime string, toolsTar string,
) (*ImageCreatorParameters, error) {
	ic := &ImageCreatorParameters{}

	// working directories
	buildDirAbs, err := filepath.Abs(buildDir)
	if err != nil {
		return nil, err
	}

	ic.buildDirAbs = buildDirAbs
	ic.toolsTar = toolsTar

	// intermediate writeable image
	ic.rawImageFile = filepath.Join(buildDirAbs, imagecustomizerlib.BaseImageName)

	// Create a uuid for the image
	imageUuid, imageUuidStr, err := randomization.CreateUuid()
	if err != nil {
		return nil, err
	}
	ic.imageUuid = imageUuid
	ic.imageUuidStr = imageUuidStr

	// configuration
	ic.configPath = configPath
	ic.config = config

	ic.rpmsSources = rpmsSources

	err = imagecustomizerlib.ValidateRpmSources(rpmsSources)
	if err != nil {
		return nil, err
	}

	// output image
	ic.outputImageFormat = imagecustomizerapi.ImageFormatType(outputImageFormat)
	if err := ic.outputImageFormat.IsValid(); err != nil {
		return nil, fmt.Errorf("invalid output image format:\n%w", err)
	}

	if ic.outputImageFormat == "" {
		ic.outputImageFormat = config.Output.Image.Format
	}

	ic.outputImageFile = outputImageFile
	if ic.outputImageFile == "" && config.Output.Image.Path != "" {
		ic.outputImageFile = file.GetAbsPathWithBase(configPath, config.Output.Image.Path)
	}

	ic.outputImageBase = strings.TrimSuffix(filepath.Base(ic.outputImageFile), filepath.Ext(ic.outputImageFile))
	ic.outputImageDir = filepath.Dir(ic.outputImageFile)
	ic.packageSnapshotTime = packageSnapshotTime

	return ic, nil
}
