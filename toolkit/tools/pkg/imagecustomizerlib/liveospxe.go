// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/tarutils"
)

func getPxeBootstrapFileUrl(bootstrapBaseUrl, bootstrapFileUrl string) (fileUrl string, err error) {

	if bootstrapBaseUrl != "" && bootstrapFileUrl != "" {
		return "", fmt.Errorf("cannot set both iso image base url and full image url at the same time")
	}

	// If the specified URL is not a full path to an iso, append the generated
	// iso file name to it.
	if bootstrapFileUrl == "" {
		fileUrl, err = url.JoinPath(bootstrapBaseUrl, defaultIsoImageName)
		if err != nil {
			return "", fmt.Errorf("failed to concatenate URL (%s) and (%s)\n%w", bootstrapBaseUrl, defaultIsoImageName, err)
		}
	} else {
		fileUrl = bootstrapFileUrl
	}

	return fileUrl, nil
}

func getPxeBootstrapFileName(bootstrapBaseUrl, bootstrapFileUrl string) (string, error) {
	bootstrapFileUrl, err := getPxeBootstrapFileUrl(bootstrapBaseUrl, bootstrapFileUrl)
	if err != nil {
		return "", err
	}
	return filepath.Base(bootstrapFileUrl), nil
}

func createPXEArtifacts(buildDir string, baseConfigPath string, initramfsType imagecustomizerapi.InitramfsImageType,
	artifactsStore *IsoArtifactsStore, additionalIsoFiles imagecustomizerapi.AdditionalFileList,
	bootstrapBaseUrl, bootstrapFileUrl, outputPath string) (err error) {
	logger.Log.Infof("Creating PXE output at (%s)", outputPath)

	outputPXEArtifactsDir := ""
	outputPXEImage := ""

	if strings.HasSuffix(outputPath, ".tar.gz") {
		// Output is a .tar.gz
		outputPXEArtifactsDir, err = os.MkdirTemp(buildDir, "tmp-pxe-")
		if err != nil {
			return fmt.Errorf("failed to create temporary mount folder for squashfs:\n%w", err)
		}
		defer os.RemoveAll(outputPXEArtifactsDir)
		outputPXEImage = outputPath
	} else {
		// Output is a folder
		outputPXEArtifactsDir = outputPath
		err := os.MkdirAll(outputPXEArtifactsDir, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create folder (%s):\n%w", outputPXEArtifactsDir, err)
		}
		outputPXEImage = ""
	}

	err = stageIsoFiles(artifactsStore.files, baseConfigPath, additionalIsoFiles, outputPXEArtifactsDir)
	if err != nil {
		return fmt.Errorf("failed to stage one or more live os files:\n%w", err)
	}

	// Move bootloader files from under '<pxe-folder>/efi/boot' to '<pxe-folder>/'
	_, bootFilesConfig, err := getBootArchConfig()
	if err != nil {
		return err
	}
	bootloaderSrcDir := filepath.Join(outputPXEArtifactsDir, isoBootloadersDir)
	bootloaderFiles := []string{bootFilesConfig.bootBinary, bootFilesConfig.grubBinary}

	for _, bootloaderFile := range bootloaderFiles {
		sourcePath := filepath.Join(bootloaderSrcDir, bootloaderFile)
		targetPath := filepath.Join(outputPXEArtifactsDir, bootloaderFile)
		err = file.Move(sourcePath, targetPath)
		if err != nil {
			return fmt.Errorf("failed to move boot loader file from (%s) to (%s) while generated the PXE artifacts folder:\n%w", sourcePath, targetPath, err)
		}
	}

	// Remove the empty 'pxe-folder>/efi' folder.
	isoEFIDir := filepath.Join(outputPXEArtifactsDir, "efi")
	err = os.RemoveAll(isoEFIDir)
	if err != nil {
		return fmt.Errorf("failed to remove folder (%s):\n%w", isoEFIDir, err)
	}

	// If bootstrap is requested, create the bootstrapped image
	if initramfsType == imagecustomizerapi.InitramfsImageTypeBootstrap {
		err = verifyDracutPXESupport(artifactsStore.info.dracutPackageInfo)
		if err != nil {
			return fmt.Errorf("failed to verify Dracut's PXE support.\n%w", err)
		}

		isoImageName, err := getPxeBootstrapFileName(bootstrapBaseUrl, bootstrapFileUrl)
		if err != nil {
			return err
		}

		// The iso image file itself must be placed in the PXE folder because
		// dracut livenet module will download it.
		artifactsIsoImagePath := filepath.Join(outputPXEArtifactsDir, isoImageName)
		err = createIsoImage(buildDir, baseConfigPath, artifactsStore.files, additionalIsoFiles, artifactsIsoImagePath)
		if err != nil {
			return fmt.Errorf("failed to create the Iso image.\n%w", err)
		}

		// The current support in dracut expects only an iso - so, no need to remove
		// the squash rootfs image.
		artifactsRootfsPath := filepath.Join(outputPXEArtifactsDir, liveOSDir, liveOSImage)
		err = os.Remove(artifactsRootfsPath)
		if err != nil {
			return fmt.Errorf("failed to remove (%s) while cleaning up intermediate files:\n%w", artifactsRootfsPath, err)
		}
	}

	// If a tar.gz is requested, create the archive
	if outputPXEImage != "" {
		err = tarutils.CreateTarGzArchive(outputPXEArtifactsDir, outputPXEImage)
		if err != nil {
			return fmt.Errorf("failed to create archive (%s) from (%s):\n%w", outputPXEImage, outputPXEArtifactsDir, err)
		}

		err = os.RemoveAll(outputPXEArtifactsDir)
		if err != nil {
			return fmt.Errorf("failed to remove (%s) while cleaning up intermediate files:\n%w", outputPXEArtifactsDir, err)
		}
	}

	return nil
}
