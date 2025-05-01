// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/isogenerator"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
)

func createInitrdImage(writeableRootfsDir, kernelVersion, outputInitrdPath string) error {
	logger.Log.Debugf("Generating initrd (%s) from (%s)", outputInitrdPath, writeableRootfsDir)

	fstabFile := filepath.Join(writeableRootfsDir, "/etc/fstab")
	logger.Log.Debugf("Deleting fstab from %s", fstabFile)
	err := os.Remove(fstabFile)
	if err != nil {
		return fmt.Errorf("failed to delete fstab:\n%w", err)
	}

	targetConfigFile := filepath.Join(writeableRootfsDir, "/etc/dracut.conf.d/20-live-cd.conf")
	err = file.Write(dracutConfig, targetConfigFile)
	if err != nil {
		return fmt.Errorf("failed to create %s:\n%w", targetConfigFile, err)
	}

	chroot := safechroot.NewChroot(writeableRootfsDir, true /*isExistingDir*/)
	if chroot == nil {
		return fmt.Errorf("failed to create a new chroot object for %s.", writeableRootfsDir)
	}
	defer chroot.Close(true /*leaveOnDisk*/)

	err = chroot.Initialize("", nil, nil, true /*includeDefaultMounts*/)
	if err != nil {
		return fmt.Errorf("failed to initialize chroot object for %s:\n%w", writeableRootfsDir, err)
	}

	requiredRpms := []string{"squashfs-tools", "tar", "device-mapper", "curl"}
	for _, requiredRpm := range requiredRpms {
		logger.Log.Debugf("Checking if (%s) is installed", requiredRpm)
		if !isPackageInstalled(chroot, requiredRpm) {
			return fmt.Errorf("package (%s) is not installed:\nthe following packages must be installed to generate an iso: %v", requiredRpm, requiredRpms)
		}
	}

	initrdPathInChroot := "/initrd.img"
	err = chroot.UnsafeRun(func() error {
		dracutParams := []string{
			initrdPathInChroot,
			"--kver", kernelVersion,
			"--filesystems", "squashfs",
			"--include", usrLibLocaleDir, usrLibLocaleDir}

		return shell.ExecuteLive(true /*squashErrors*/, "dracut", dracutParams...)
	})
	if err != nil {
		return fmt.Errorf("failed to run dracut:\n%w", err)
	}

	generatedInitrdPath := filepath.Join(writeableRootfsDir, initrdPathInChroot)
	err = file.Move(generatedInitrdPath, outputInitrdPath)
	if err != nil {
		return fmt.Errorf("failed to copy generated initrd:\n%w", err)
	}

	return nil
}

func createSquashfsImage(writeableRootfsDir, outputSquashfsPath string) error {
	logger.Log.Debugf("Creating squashfs (%s) from (%s)", outputSquashfsPath, writeableRootfsDir)

	err := os.RemoveAll(filepath.Join(writeableRootfsDir, "boot"))
	if err != nil {
		return fmt.Errorf("failed to remove the /boot folder from the source image:\n%w", err)
	}

	exists, err := file.PathExists(outputSquashfsPath)
	if err == nil && exists {
		err = os.Remove(outputSquashfsPath)
		if err != nil {
			return fmt.Errorf("failed to delete existing squashfs image (%s):\n%w", outputSquashfsPath, err)
		}
	}

	// '-xattrs' allows SELinux labeling to be retained within the squashfs.
	mksquashfsParams := []string{writeableRootfsDir, outputSquashfsPath, "-xattrs"}
	err = shell.ExecuteLive(true, "mksquashfs", mksquashfsParams...)
	if err != nil {
		return fmt.Errorf("failed to create squashfs:\n%w", err)
	}

	return nil
}

func stageIsoFile(sourcePath, stageDirPath, isoRelativeDir string) error {
	targetPath := filepath.Join(stageDirPath, isoRelativeDir, filepath.Base(sourcePath))
	targetDir := filepath.Dir(targetPath)
	err := os.MkdirAll(targetDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create destination directory (%s):\n%w", targetDir, err)
	}

	err = file.Copy(sourcePath, targetPath)
	if err != nil {
		return fmt.Errorf("failed to stage file from (%s) to (%s):\n%w", sourcePath, targetPath, err)
	}

	return nil
}

func stageIsoFiles(filesStore *IsoFilesStore, baseConfigPath string, additionalIsoFiles imagecustomizerapi.AdditionalFileList,
	stagingDir string,
) error {
	err := os.RemoveAll(stagingDir)
	if err != nil {
		return err
	}

	err = os.MkdirAll(stagingDir, os.ModePerm)
	if err != nil {
		return err
	}

	// map of file full local path to location on iso media.
	artifactsToIsoMap := map[string]string{
		filesStore.isoBootImagePath:  "boot/grub2",
		filesStore.isoGrubCfgPath:    "boot/grub2",
		filesStore.vmlinuzPath:       "boot",
		filesStore.initrdImagePath:   "boot",
		filesStore.squashfsImagePath: "liveos",
	}

	// Add optional saved config file if it exists.
	if filesStore.savedConfigsFilePath != "" {
		exists, err := file.PathExists(filesStore.savedConfigsFilePath)
		if err != nil {
			return fmt.Errorf("failed to check if (%s) exists:\n%w", filesStore.savedConfigsFilePath, err)
		}
		if exists {
			artifactsToIsoMap[filesStore.savedConfigsFilePath] = "azl-image-customizer"
		}
	}

	// Add optional grub-pxe.cfg file if it exists.
	if filesStore.pxeGrubCfgPath != "" {
		exists, err := file.PathExists(filesStore.pxeGrubCfgPath)
		if err != nil {
			return fmt.Errorf("failed to check if (%s) exists:\n%w", filesStore.pxeGrubCfgPath, err)
		}
		if exists {
			artifactsToIsoMap[filesStore.pxeGrubCfgPath] = "boot/grub2"
		}
	}

	// Add additional files
	// - This is typically populated if a previous run added additional files.
	for source, isoRelativePath := range filesStore.additionalFiles {
		isoRelativeDir := filepath.Dir(isoRelativePath)
		artifactsToIsoMap[source] = isoRelativeDir
	}

	// Stage the files
	for source, isoRelativeDir := range artifactsToIsoMap {
		err = stageIsoFile(source, stagingDir, isoRelativeDir)
		if err != nil {
			return fmt.Errorf("failed to stage (%s):\n%w", source, err)
		}
	}

	// Stage config-defined additional files
	// - This is typically populated if the current configuration defines
	//   additional files.
	var filesToCopy []safechroot.FileToCopy
	for _, additionalFile := range additionalIsoFiles {
		absSourceFile := ""
		if additionalFile.Source != "" {
			absSourceFile = file.GetAbsPathWithBase(baseConfigPath, additionalFile.Source)
		}

		fileToCopy := safechroot.FileToCopy{
			Src:         absSourceFile,
			Content:     additionalFile.Content,
			Dest:        additionalFile.Destination,
			Permissions: (*fs.FileMode)(additionalFile.Permissions),
		}
		filesToCopy = append(filesToCopy, fileToCopy)
	}

	err = safechroot.AddFilesToDestination(stagingDir, filesToCopy...)
	if err != nil {
		return fmt.Errorf("failed to stage config-defined additional files:\n%w", err)
	}

	// Apply Rufus workaround
	err = isogenerator.ApplyRufusWorkaround(filesStore.bootEfiPath, filesStore.grubEfiPath, stagingDir)
	if err != nil {
		return fmt.Errorf("failed to apply Rufus work-around:\n%w", err)
	}

	return nil
}

func createIsoImage(buildDir string, filesStore *IsoFilesStore, baseConfigPath string,
	additionalIsoFiles imagecustomizerapi.AdditionalFileList, outputImagePath string) error {
	stagingDir := filepath.Join(buildDir, "staging")

	err := stageIsoFiles(filesStore, baseConfigPath, additionalIsoFiles, stagingDir)
	if err != nil {
		return fmt.Errorf("failed to stage one or more iso files:\n%w", err)
	}

	biosBootEnabled := false
	biosFilesDirPath := ""
	err = isogenerator.BuildIsoImage(stagingDir, biosBootEnabled, biosFilesDirPath, outputImagePath)
	if err != nil {
		return fmt.Errorf("failed to create (%s) using the (%s) folder:\n%w", outputImagePath, stagingDir, err)
	}

	return nil
}
