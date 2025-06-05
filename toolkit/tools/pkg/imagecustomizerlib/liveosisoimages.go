// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/initrdutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/isogenerator"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safemount"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/targetos"
)

const (

	// ToDo: this is not being invoked...
	initScriptFileName = "init"
	// Having #!/bin/bash header causes a kernel panic.
	// Not having the init file at all, causes a kernel panic.
	initContent = `mount -t proc proc /proc
/lib/systemd/systemd
`

	dracutConfig = `add_dracutmodules+=" dmsquash-live livenet selinux "
add_drivers+=" overlay "
hostonly="no"
`

	// the total size of a collection of files is multiplied by the
	// expansionSafetyFactor to estimate a disk size sufficient to hold those
	// files.
	expansionSafetyFactor = 1.5

	// This folder is necessary to include in the initrd image so that the
	// emergency shell can work correctly with the keyboard.
	usrLibLocaleDir = "/usr/lib/locale"
)

type StageFile struct {
	sourcePath    string
	targetRelPath string
	targetName    string
}

func cleanFullOSFolderForLiveOS(fullOSDir string) error {
	fstabFile := filepath.Join(fullOSDir, "/etc/fstab")
	logger.Log.Debugf("Deleting fstab from %s", fstabFile)

	err := os.Remove(fstabFile)
	if err != nil {
		return fmt.Errorf("failed to delete fstab:\n%w", err)
	}

	logger.Log.Debugf("Deleting /boot")
	err = os.RemoveAll(filepath.Join(fullOSDir, "boot"))
	if err != nil {
		return fmt.Errorf("failed to remove the /boot folder from the source image:\n%w", err)
	}

	return nil
}

func createFullOSInitrdImage(writeableRootfsDir, outputInitrdPath string) error {
	logger.Log.Infof("Creating full OS initrd")

	err := cleanFullOSFolderForLiveOS(writeableRootfsDir)
	if err != nil {
		return fmt.Errorf("failed to clean root filesystem directory (%s):\n%w", writeableRootfsDir, err)
	}

	initScriptPath := filepath.Join(writeableRootfsDir, initScriptFileName)
	err = os.WriteFile(initScriptPath, []byte(initContent), 0755)
	if err != nil {
		return fmt.Errorf("failed to create (%s):\n%w", initScriptPath, err)
	}

	err = initrdutils.CreateInitrdImageFromFolder(writeableRootfsDir, outputInitrdPath)
	if err != nil {
		return fmt.Errorf("failed to create the initrd image:\n%w", err)
	}

	return nil
}

func createBootstrapInitrdImage(writeableRootfsDir, kernelVersion, outputInitrdPath string) error {
	logger.Log.Infof("Creating bootstrap initrd")

	dracutConfigFile := filepath.Join(writeableRootfsDir, "/etc/dracut.conf.d/20-live-cd.conf")
	err := file.Write(dracutConfig, dracutConfigFile)
	if err != nil {
		return fmt.Errorf("failed to create %s:\n%w", dracutConfigFile, err)
	}
	defer os.Remove(dracutConfigFile)

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
	logger.Log.Infof("Creating squashfs")

	err := cleanFullOSFolderForLiveOS(writeableRootfsDir)
	if err != nil {
		return fmt.Errorf("failed to clean root filesystem directory (%s):\n%w", writeableRootfsDir, err)
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

func stageLiveOSFile(stageDirPath string, stageFile StageFile) error {
	targetPath := ""
	if stageFile.targetName == "" {
		targetPath = filepath.Join(stageDirPath, stageFile.targetRelPath, filepath.Base(stageFile.sourcePath))
	} else {
		targetPath = filepath.Join(stageDirPath, stageFile.targetRelPath, stageFile.targetName)
	}
	targetDir := filepath.Dir(targetPath)
	err := os.MkdirAll(targetDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create destination directory (%s):\n%w", targetDir, err)
	}

	err = file.Copy(stageFile.sourcePath, targetPath)
	if err != nil {
		return fmt.Errorf("failed to stage file from (%s) to (%s):\n%w", stageFile.sourcePath, targetPath, err)
	}

	return nil
}

func stageLiveOSFiles(outputFormat imagecustomizerapi.ImageFormatType, filesStore *IsoFilesStore, baseConfigPath string,
	additionalIsoFiles imagecustomizerapi.AdditionalFileList, stagingDir string,
) error {
	err := os.RemoveAll(stagingDir)
	if err != nil {
		return err
	}

	err = os.MkdirAll(stagingDir, os.ModePerm)
	if err != nil {
		return err
	}

	artifactsToLiveOSMap := []StageFile{
		{
			sourcePath:    filesStore.isoBootImagePath,
			targetRelPath: "boot/grub2",
		},
		{
			sourcePath:    filesStore.vmlinuzPath,
			targetRelPath: "boot",
		},
		{
			sourcePath:    filesStore.initrdImagePath,
			targetRelPath: "boot",
		},
	}

	switch outputFormat {
	case imagecustomizerapi.ImageFormatTypeIso:
		artifactsToLiveOSMap = append(artifactsToLiveOSMap,
			StageFile{
				sourcePath:    filesStore.isoGrubCfgPath,
				targetRelPath: "boot/grub2",
				targetName:    "grub.cfg",
			})
	case imagecustomizerapi.ImageFormatTypePxeDir, imagecustomizerapi.ImageFormatTypePxeTar:
		artifactsToLiveOSMap = append(artifactsToLiveOSMap,
			StageFile{
				sourcePath:    filesStore.pxeGrubCfgPath,
				targetRelPath: "boot/grub2",
				targetName:    "grub.cfg",
			})
	default:
		return fmt.Errorf("unsupported output format while staging file for Live OS output:\n%v", outputFormat)
	}

	// Add optional squashfs file if it exists.
	if filesStore.squashfsImagePath != "" {
		exists, err := file.PathExists(filesStore.squashfsImagePath)
		if err != nil {
			return fmt.Errorf("failed to check if (%s) exists:\n%w", filesStore.squashfsImagePath, err)
		}
		if exists {
			artifactsToLiveOSMap = append(artifactsToLiveOSMap,
				StageFile{
					sourcePath:    filesStore.squashfsImagePath,
					targetRelPath: liveOSDir,
				})
		}
	}

	// Add optional saved config file if it exists.
	if filesStore.savedConfigsFilePath != "" {
		exists, err := file.PathExists(filesStore.savedConfigsFilePath)
		if err != nil {
			return fmt.Errorf("failed to check if (%s) exists:\n%w", filesStore.savedConfigsFilePath, err)
		}
		if exists {
			artifactsToLiveOSMap = append(artifactsToLiveOSMap,
				StageFile{
					sourcePath:    filesStore.savedConfigsFilePath,
					targetRelPath: savedConfigsDir,
				})
		}
	}

	// Add additional files
	// - This is typically populated if a previous run added additional files.
	for source, isoRelativePath := range filesStore.additionalFiles {
		isoRelativeDir := filepath.Dir(isoRelativePath)
		artifactsToLiveOSMap = append(artifactsToLiveOSMap,
			StageFile{
				sourcePath:    source,
				targetRelPath: isoRelativeDir,
			})
	}

	// Stage the files
	for _, stageFile := range artifactsToLiveOSMap {
		err = stageLiveOSFile(stagingDir, stageFile)
		if err != nil {
			return fmt.Errorf("failed to stage (%s):\n%w", stageFile.sourcePath, err)
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

func createIsoImage(buildDir string, baseConfigPath string, filesStore *IsoFilesStore,
	additionalIsoFiles imagecustomizerapi.AdditionalFileList, outputImagePath string) error {
	stagingDir := filepath.Join(buildDir, "iso-staging")

	err := stageLiveOSFiles(imagecustomizerapi.ImageFormatTypeIso, filesStore, baseConfigPath, additionalIsoFiles, stagingDir)
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

func getSizeOnDiskInBytes(fileOrDir string) (size uint64, err error) {
	logger.Log.Debugf("Calculating total size for (%s)", fileOrDir)

	duStdout, _, err := shell.Execute("du", "-s", fileOrDir)
	if err != nil {
		return 0, fmt.Errorf("failed to find the size of the specified file/dir using 'du' for (%s):\n%w", fileOrDir, err)
	}

	// parse and get count and unit
	diskSizeRegex := regexp.MustCompile(`^(\d+)\s+`)
	matches := diskSizeRegex.FindStringSubmatch(duStdout)
	if matches == nil || len(matches) < 2 {
		return 0, fmt.Errorf("failed to parse 'du -s' output (%s).", duStdout)
	}

	sizeInKbsString := matches[1]
	sizeInKbs, err := strconv.ParseUint(sizeInKbsString, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse disk size (%d):\n%w", sizeInKbs, err)
	}

	return sizeInKbs * diskutils.KiB, nil
}

func getDiskSizeEstimateInMBs(filesOrDirs []string, safetyFactor float64) (size uint64, err error) {

	totalSizeInBytes := uint64(0)
	for _, fileOrDir := range filesOrDirs {
		sizeInBytes, err := getSizeOnDiskInBytes(fileOrDir)
		if err != nil {
			return 0, fmt.Errorf("failed to get the size of (%s) on disk while estimating total disk size:\n%w", fileOrDir, err)
		}
		totalSizeInBytes += sizeInBytes
	}

	sizeInMBs := totalSizeInBytes/diskutils.MiB + 1
	estimatedSizeInMBs := uint64(float64(sizeInMBs) * safetyFactor)
	return estimatedSizeInMBs, nil
}

func createWriteableImageFromArtifacts(buildDir string, artifactsStore *IsoArtifactsStore, rawImageFile string) error {
	logger.Log.Infof("Creating full OS writeable image from ISO artifacts")

	rootfsDir, err := os.MkdirTemp(buildDir, "tmp-full-os-root-")
	if err != nil {
		return fmt.Errorf("failed to create temporary mount folder for squashfs:\n%w", err)
	}
	defer os.RemoveAll(rootfsDir)

	squashfsExists, err := file.PathExists(artifactsStore.files.squashfsImagePath)
	if err != nil {
		return fmt.Errorf("failed to check if the squash root file system image exists (%s):\n%w", artifactsStore.files.squashfsImagePath, err)
	}

	var squashfsLoopDevice *safeloopback.Loopback
	var squashfsMount *safemount.Mount

	if squashfsExists {
		logger.Log.Infof("Detected bootstrap OS initrd configuration")
		squashfsLoopDevice, err = safeloopback.NewLoopback(artifactsStore.files.squashfsImagePath)
		if err != nil {
			return fmt.Errorf("failed to create loop device for (%s):\n%w", artifactsStore.files.squashfsImagePath, err)
		}
		defer squashfsLoopDevice.Close()

		squashfsMount, err = safemount.NewMount(squashfsLoopDevice.DevicePath(), rootfsDir,
			"squashfs" /*fstype*/, 0 /*flags*/, "" /*data*/, false /*makeAndDelete*/)
		if err != nil {
			return err
		}
		defer squashfsMount.Close()
	} else {
		logger.Log.Infof("Detected full OS initrd configuration")
		err = initrdutils.CreateFolderFromInitrdImage(artifactsStore.files.initrdImagePath, rootfsDir)
		if err != nil {
			return fmt.Errorf("failed to extract files from the initrd image (%s):\n%w", artifactsStore.files.initrdImagePath, err)
		}
	}

	logger.Log.Infof("Populated (%s) with full file system", rootfsDir)

	// boot folder (from artifacts)
	artifactsBootDir := filepath.Join(artifactsStore.files.artifactsDir, "boot")

	imageContentList := []string{
		rootfsDir,
		artifactsStore.files.bootEfiPath,
		artifactsStore.files.grubEfiPath,
		artifactsBootDir}

	// estimate the new disk size
	safeDiskSizeMB, err := getDiskSizeEstimateInMBs(imageContentList, expansionSafetyFactor)
	if err != nil {
		return fmt.Errorf("failed to calculate the disk size of %s:\n%w", rootfsDir, err)
	}

	logger.Log.Debugf("safeDiskSizeMB = %d", safeDiskSizeMB)

	// define a disk layout with a boot partition and a rootfs partition
	maxDiskSizeMB := imagecustomizerapi.DiskSize(safeDiskSizeMB * diskutils.MiB)
	bootPartitionStart := imagecustomizerapi.DiskSize(1 * diskutils.MiB)
	bootPartitionEnd := imagecustomizerapi.DiskSize(9 * diskutils.MiB)

	diskConfig := imagecustomizerapi.Disk{
		PartitionTableType: imagecustomizerapi.PartitionTableTypeGpt,
		MaxSize:            &maxDiskSizeMB,
		Partitions: []imagecustomizerapi.Partition{
			{
				Id:    "esp",
				Start: &bootPartitionStart,
				End:   &bootPartitionEnd,
				Type:  imagecustomizerapi.PartitionTypeESP,
			},
			{
				Id:    "rootfs",
				Start: &bootPartitionEnd,
			},
		},
	}

	fileSystemConfigs := []imagecustomizerapi.FileSystem{
		{
			DeviceId:    "esp",
			PartitionId: "esp",
			Type:        imagecustomizerapi.FileSystemTypeFat32,
			MountPoint: &imagecustomizerapi.MountPoint{
				Path:    "/boot/efi",
				Options: "umask=0077",
			},
		},
		{
			DeviceId:    "rootfs",
			PartitionId: "rootfs",
			Type:        imagecustomizerapi.FileSystemTypeExt4,
			MountPoint: &imagecustomizerapi.MountPoint{
				Path: "/",
			},
		},
	}

	targetOs, err := targetos.GetInstalledTargetOs(rootfsDir)
	if err != nil {
		return fmt.Errorf("failed to determine target OS of ISO squashfs:\n%w", err)
	}

	// populate the newly created disk image with content from the squash fs
	installOSFunc := func(imageChroot *safechroot.Chroot) error {
		logger.Log.Infof("Installing files to empty image")
		// At the point when this copy will be executed, both the boot and the
		// root partitions will be mounted, and the files of /boot/efi will
		// land on the the boot partition, while the rest will be on the rootfs
		// partition.
		err := copyPartitionFiles(rootfsDir+"/.", imageChroot.RootDir())
		if err != nil {
			return fmt.Errorf("failed to copy squashfs contents to a writeable disk:\n%w", err)
		}

		// Note that before the LiveOS ISO is first created, the boot folder is
		// removed from the squashfs since it is not needed. The boot artifacts
		// are stored directly on the ISO media outside the squashfs image.
		// Now that we are re-constructing the full file system, we need to
		// pull the boot artifacts back into the full file system so that
		// it is restored to its original state and subsequent customization
		// or extraction can proceed transparently.
		err = copyPartitionFiles(artifactsBootDir, imageChroot.RootDir())
		if err != nil {
			return fmt.Errorf("failed to copy (%s) contents to a writeable disk:\n%w", artifactsBootDir, err)
		}

		// The `initrd.img` must be on the form `initrd-*` so that `grub2-mkconfig`
		// can find it. If it cannot find it, the generated grub.cfg will be missing
		// all the boot entries.
		initrdFileName := fmt.Sprintf("initrd-%s.img", artifactsStore.info.kernelVersion)
		initrdOld := filepath.Join(imageChroot.RootDir(), "boot/initrd.img")
		initrdNew := filepath.Join(imageChroot.RootDir(), "boot", initrdFileName)
		err = os.Rename(initrdOld, initrdNew)
		if err != nil {
			return fmt.Errorf("failed to rename (%s) to (%s)", initrdOld, initrdNew)
		}

		// The `vmlinuz` must be on the form `vmlinuz-*` so that `grub2-mkconfig`
		// can find it. If it cannot find it, the generated grub.cfg will be missing
		// all the boot entries.
		kernelFileName := fmt.Sprintf("vmlinuz-%s", artifactsStore.info.kernelVersion)
		kernelOld := filepath.Join(imageChroot.RootDir(), "boot/vmlinuz")
		kernelNew := filepath.Join(imageChroot.RootDir(), "boot", kernelFileName)
		err = os.Rename(kernelOld, kernelNew)
		if err != nil {
			return fmt.Errorf("failed to rename (%s) to (%s)", kernelOld, kernelNew)
		}

		targetEfiDir := filepath.Join(imageChroot.RootDir(), "boot/efi/EFI/BOOT")
		err = os.MkdirAll(targetEfiDir, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create destination efi directory (%s):\n%w", targetEfiDir, err)
		}

		targetShimPath := filepath.Join(targetEfiDir, filepath.Base(artifactsStore.files.bootEfiPath))
		err = file.Copy(artifactsStore.files.bootEfiPath, targetShimPath)
		if err != nil {
			return fmt.Errorf("failed to copy (%s) to (%s):\n%w", artifactsStore.files.bootEfiPath, targetShimPath, err)
		}

		targetGrubPath := filepath.Join(targetEfiDir, filepath.Base(artifactsStore.files.grubEfiPath))
		err = file.Copy(artifactsStore.files.grubEfiPath, targetGrubPath)
		if err != nil {
			return fmt.Errorf("failed to copy (%s) to (%s):\n%w", artifactsStore.files.grubEfiPath, targetGrubPath, err)
		}

		return err
	}

	// create the new raw disk image
	writeableChrootDir := "writeable-raw-image"
	_, err = createNewImage(targetOs, rawImageFile, diskConfig, fileSystemConfigs, buildDir, writeableChrootDir,
		installOSFunc)
	if err != nil {
		return fmt.Errorf("failed to copy squashfs into new writeable image (%s):\n%w", rawImageFile, err)
	}

	if squashfsMount != nil {
		err = squashfsMount.CleanClose()
		if err != nil {
			return err
		}
	}

	if squashfsLoopDevice != nil {
		err = squashfsLoopDevice.CleanClose()
		if err != nil {
			return err
		}
	}

	return nil
}
