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

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/initrdutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/isogenerator"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
)

const (
	bootDirPermissions = 0o700

	initKernelInitrdPath = "/init"
	initBinaryInitrdPath = "/lib/systemd/systemd"

	dracutConfig = `add_dracutmodules+=" dmsquash-live livenet selinux "
add_drivers+=" overlay "
hostonly="no"
`
	vmlinuzPrefix = "vmlinuz-"

	// the total size of a collection of files is multiplied by the
	// expansionSafetyFactor to estimate a disk size sufficient to hold those
	// files.
	expansionSafetyFactor = 1.5

	// This folder is necessary to include in the initrd image so that the
	// emergency shell can work correctly with the keyboard.
	usrLibLocaleDir = "/usr/lib/locale"
)

var (
	kdumpInitramfsRegEx = regexp.MustCompile(`/initramfs-(.*)kdump\.img$`)
)

type StageFile struct {
	sourcePath    string
	targetRelPath string
	targetName    string
}

func cleanFullOSFolderForLiveOS(fullOSDir string, kdumpBootFiles *imagecustomizerapi.KdumpBootFilesType,
	kdumpBootFilesMap map[string]*KdumpBootFiles,
) error {
	fstabFile := filepath.Join(fullOSDir, "/etc/fstab")
	logger.Log.Debugf("Deleting fstab from %s", fstabFile)

	err := os.Remove(fstabFile)
	if err != nil {
		return fmt.Errorf("failed to delete fstab:\n%w", err)
	}

	logger.Log.Infof("Deleting /boot")
	bootFolder := filepath.Join(fullOSDir, "boot")
	err = os.RemoveAll(bootFolder)
	if err != nil {
		return fmt.Errorf("failed to remove the /boot folder from the source image:\n%w", err)
	}

	keepKdumpBootFiles := false
	if kdumpBootFiles != nil && *kdumpBootFiles == imagecustomizerapi.KdumpBootFilesTypeKeep {
		keepKdumpBootFiles = true
	}

	// Restore kdump files if desired and they exist
	if keepKdumpBootFiles && kdumpBootFilesMap != nil && len(kdumpBootFilesMap) > 0 {
		err := os.MkdirAll(bootFolder, bootDirPermissions)
		if err != nil {
			return fmt.Errorf("failed to re-create directory (%s):\n%w", bootFolder, err)
		}

		for _, kdumpBootFiles := range kdumpBootFilesMap {
			// Note that kdump boot files pair is created if the initramfs*kdump.img
			// file is found. There is a possibility that the initramfs*kdump.img is
			// found, but the corresponding kernel file is not. So, we need to check
			// if it has really been found, before copying it.
			if kdumpBootFiles.vmlinuzPath != "" {
				logger.Log.Infof("Restoring %s", kdumpBootFiles.vmlinuzPath)
				restoredFilePath := filepath.Join(bootFolder, filepath.Base(kdumpBootFiles.vmlinuzPath))
				err := file.Copy(kdumpBootFiles.vmlinuzPath, restoredFilePath)
				if err != nil {
					return fmt.Errorf("failed to copy (%s) to (%s):\n%w", kdumpBootFiles.vmlinuzPath, restoredFilePath, err)
				}
			}
			logger.Log.Infof("Restoring %s", kdumpBootFiles.initrdImagePath)
			restoredFilePath := filepath.Join(bootFolder, filepath.Base(kdumpBootFiles.initrdImagePath))
			err = file.Copy(kdumpBootFiles.initrdImagePath, restoredFilePath)
			if err != nil {
				return fmt.Errorf("failed to copy (%s) to (%s):\n%w", kdumpBootFiles.initrdImagePath, restoredFilePath, err)
			}
		}
	}

	return nil
}

func createFullOSInitrdImage(writeableRootfsDir string, kernelKdumpFiles *imagecustomizerapi.KdumpBootFilesType,
	kdumpBootFilesMap map[string]*KdumpBootFiles, outputInitrdPath string) error {
	logger.Log.Infof("Creating full OS initrd")

	err := cleanFullOSFolderForLiveOS(writeableRootfsDir, kernelKdumpFiles, kdumpBootFilesMap)
	if err != nil {
		return fmt.Errorf("failed to clean root filesystem directory (%s):\n%w", writeableRootfsDir, err)
	}

	initKernelLocalPath := filepath.Join(writeableRootfsDir, initKernelInitrdPath)

	exists, err := file.PathExists(initKernelLocalPath)
	if err != nil {
		return fmt.Errorf("failed to check if (%s) exists:\n%w", initKernelLocalPath, err)
	}
	if !exists {
		err = os.Symlink(initBinaryInitrdPath, initKernelLocalPath)
		if err != nil {
			return fmt.Errorf("failed to create symlink (%s):\n%w", initKernelLocalPath, err)
		}
	}

	err = initrdutils.CreateInitrdImageFromFolder(writeableRootfsDir, outputInitrdPath)
	if err != nil {
		return fmt.Errorf("failed to create the initrd image:\n%w", err)
	}

	return nil
}

func createBootstrapInitrdImage(writeableRootfsDir, kernelVersion, outputInitrdPath string) error {
	logger.Log.Infof("Creating bootstrap initrd for %s", kernelVersion)

	dracutConfigFile := filepath.Join(writeableRootfsDir, "/etc/dracut.conf.d/20-live-cd.conf")
	err := file.Write(dracutConfig, dracutConfigFile)
	if err != nil {
		return fmt.Errorf("failed to create %s:\n%w", dracutConfigFile, err)
	}
	defer os.Remove(dracutConfigFile)

	chroot := safechroot.NewChroot(writeableRootfsDir, true /*isExistingDir*/)
	if chroot == nil {
		return fmt.Errorf("failed to create a new chroot object for (%s)", writeableRootfsDir)
	}
	defer chroot.Close(true /*leaveOnDisk*/)

	err = chroot.Initialize("", nil, nil, true /*includeDefaultMounts*/)
	if err != nil {
		return fmt.Errorf("failed to initialize chroot object for %s:\n%w", writeableRootfsDir, err)
	}

	distroHandler, err := getDistroHandlerFromChroot(chroot)
	if err != nil {
		return fmt.Errorf("failed to determine distro from chroot:\n%w", err)
	}

	requiredRpms := []string{"squashfs-tools", "tar", "device-mapper", "curl"}
	for _, requiredRpm := range requiredRpms {
		logger.Log.Debugf("Checking if (%s) is installed", requiredRpm)
		if !distroHandler.isPackageInstalled(chroot, requiredRpm) {
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

func createSquashfsImage(writeableRootfsDir string, kdumpBootFiles *imagecustomizerapi.KdumpBootFilesType,
	kdumpBootFilesMap map[string]*KdumpBootFiles, outputSquashfsPath string) error {
	logger.Log.Infof("Creating squashfs")

	err := cleanFullOSFolderForLiveOS(writeableRootfsDir, kdumpBootFiles, kdumpBootFilesMap)
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

func stageLiveOSFiles(initramfsType imagecustomizerapi.InitramfsImageType, outputFormat imagecustomizerapi.ImageFormatType,
	filesStore *IsoFilesStore, baseConfigPath string, kdumpBootFiles *imagecustomizerapi.KdumpBootFilesType,
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

	artifactsToLiveOSMap := []StageFile{}

	for _, kernelFiles := range filesStore.kernelBootFiles {
		artifactsToLiveOSMap = append(artifactsToLiveOSMap,
			StageFile{
				sourcePath:    kernelFiles.vmlinuzPath,
				targetRelPath: "boot",
			})

		for _, otherKernelFile := range kernelFiles.otherFiles {
			artifactsToLiveOSMap = append(artifactsToLiveOSMap,
				StageFile{
					sourcePath:    otherKernelFile,
					targetRelPath: "boot",
				})
		}
	}

	// If kdump boot files are not kept under the /boot folder in the full OS
	// image, we need to include them directly on the Live OS media or else we
	// lose them completely.
	if kdumpBootFiles != nil && *kdumpBootFiles == imagecustomizerapi.KdumpBootFilesTypeNone {
		for _, kernelFiles := range filesStore.kdumpBootFiles {
			artifactsToLiveOSMap = append(artifactsToLiveOSMap,
				StageFile{
					sourcePath:    kernelFiles.vmlinuzPath,
					targetRelPath: "boot",
				})
			artifactsToLiveOSMap = append(artifactsToLiveOSMap,
				StageFile{
					sourcePath:    kernelFiles.initrdImagePath,
					targetRelPath: "boot",
				})
		}
	}

	switch initramfsType {
	case imagecustomizerapi.InitramfsImageTypeFullOS:
		artifactsToLiveOSMap = append(artifactsToLiveOSMap,
			StageFile{
				sourcePath:    filesStore.initrdImagePath,
				targetRelPath: "boot",
			})
	case imagecustomizerapi.InitramfsImageTypeBootstrap:
		for _, kernelBootFiles := range filesStore.kernelBootFiles {
			artifactsToLiveOSMap = append(artifactsToLiveOSMap,
				StageFile{
					sourcePath:    kernelBootFiles.initrdImagePath,
					targetRelPath: "boot",
				})
		}
	}

	switch outputFormat {
	case imagecustomizerapi.ImageFormatTypeIso:
		artifactsToLiveOSMap = append(artifactsToLiveOSMap,
			StageFile{
				sourcePath:    filesStore.isoBootImagePath,
				targetRelPath: "boot/grub2",
			})
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

func createIsoImage(buildDir string, baseConfigPath string, initramfsType imagecustomizerapi.InitramfsImageType, filesStore *IsoFilesStore,
	kdumpBootFiles *imagecustomizerapi.KdumpBootFilesType, additionalIsoFiles imagecustomizerapi.AdditionalFileList,
	outputImagePath string) error {
	stagingDir := filepath.Join(buildDir, "iso-staging")

	err := stageLiveOSFiles(initramfsType, imagecustomizerapi.ImageFormatTypeIso, filesStore,
		baseConfigPath, kdumpBootFiles, additionalIsoFiles, stagingDir)
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
		return 0, fmt.Errorf("failed to parse 'du -s' output (%s)", duStdout)
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

func createWriteableImageFromArtifacts(buildDir string, inputArtifactsStore *IsoArtifactsStore, rawImageFile string) error {
	logger.Log.Infof("Creating full OS writeable image from ISO artifacts")

	rootfsDir, err := os.MkdirTemp(buildDir, "tmp-full-os-root-")
	if err != nil {
		return fmt.Errorf("failed to create temporary mount folder for squashfs:\n%w", err)
	}
	defer os.RemoveAll(rootfsDir)

	squashfsExists, err := file.PathExists(inputArtifactsStore.files.squashfsImagePath)
	if err != nil {
		return fmt.Errorf("failed to check if the squash root file system image exists (%s):\n%w", inputArtifactsStore.files.squashfsImagePath, err)
	}

	var squashfsLoopDevice *safeloopback.Loopback
	var squashfsMount *safemount.Mount

	if squashfsExists {
		logger.Log.Infof("Detected bootstrap OS initrd configuration")
		squashfsLoopDevice, err = safeloopback.NewLoopback(inputArtifactsStore.files.squashfsImagePath)
		if err != nil {
			return fmt.Errorf("failed to create loop device for (%s):\n%w", inputArtifactsStore.files.squashfsImagePath, err)
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
		err = initrdutils.CreateFolderFromInitrdImage(inputArtifactsStore.files.initrdImagePath, rootfsDir)
		if err != nil {
			return fmt.Errorf("failed to extract files from the initrd image (%s):\n%w", inputArtifactsStore.files.initrdImagePath, err)
		}
	}

	logger.Log.Debugf("Populated (%s) with full file system", rootfsDir)

	// boot folder (from artifacts)
	artifactsBootDir := filepath.Join(inputArtifactsStore.files.artifactsDir, "boot")

	imageContentList := []string{
		rootfsDir,
		inputArtifactsStore.files.bootEfiPath,
		inputArtifactsStore.files.grubEfiPath,
		artifactsBootDir}

	// estimate the new disk size
	safeDiskSizeMB, err := getDiskSizeEstimateInMBs(imageContentList, expansionSafetyFactor)
	if err != nil {
		return fmt.Errorf("failed to calculate the disk size of %s:\n%w", rootfsDir, err)
	}

	logger.Log.Debugf("safeDiskSizeMB = %d", safeDiskSizeMB)

	// define a disk layout with a boot partition and a rootfs partition
	maxDiskSizeMB := imagecustomizerapi.DiskSize(safeDiskSizeMB * diskutils.MiB)
	partitionStart := imagecustomizerapi.DiskSize(1 * diskutils.MiB)

	diskConfig := imagecustomizerapi.Disk{
		PartitionTableType: imagecustomizerapi.PartitionTableTypeGpt,
		MaxSize:            &maxDiskSizeMB,
		Partitions: []imagecustomizerapi.Partition{
			{
				Id:    "rootfs",
				Start: &partitionStart,
			},
		},
	}

	fileSystemConfigs := []imagecustomizerapi.FileSystem{
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
		//
		// We are disabling the --no-clobber option since the target boot
		// folder may have common files already from the previous steps.
		// This can happen when kdump boot files are set to 'keep' in the
		// input iso which will result in two copies of the kernel vmlinux
		// file:
		// - one from the iso media
		// - one from the full OS image (next to the initramfs kdump.img file).
		// When the OS is being re-constructed, the attempt to re-copy the
		// kernel vmlinuz file will fail if we use --no-clobber. So, we disable
		// it here while coying the boot files from the artifacts.
		err = copyPartitionFilesWithOptions(artifactsBootDir, imageChroot.RootDir(), false /*noClobber*/)
		if err != nil {
			return fmt.Errorf("failed to copy (%s) contents to a writeable disk:\n%w", artifactsBootDir, err)
		}

		initrdDir := filepath.Join(imageChroot.RootDir(), "boot")
		for kernelVersion, kernelBootFiles := range inputArtifactsStore.files.kernelBootFiles {
			// The `initrd.img` must be on the form `initrd-*` so that `grub2-mkconfig`
			// can find it. If it cannot find it, the generated grub.cfg will be missing
			// all the boot entries.
			if kernelBootFiles.initrdImagePath == "" {
				kernelBootFiles.initrdImagePath = filepath.Join(initrdDir, "initramfs-"+kernelVersion+".img")
			}
			exists, err := file.PathExists(kernelBootFiles.initrdImagePath)
			if err != nil {
				return fmt.Errorf("failed to check if (%s) exists:\n%w", kernelBootFiles.initrdImagePath, err)
			}
			if !exists {
				// The input image might be coming from a full OS initramfs in
				// which case there is a single initramfs for all kernels used.
				// When grub-mkconfig is run to generate grub.cfg, it looks for
				// kernel/initramfs pairs, and if initramfs for a given kernel
				// version is missing, it does not generate a grub.cfg entry
				// for that combination.
				// To avoid this, we are creating dummy initramfs files to be
				// placeholders so that grub.cfg is generated correctly.
				// If the final output is a bootstrap initramfs, the dummy files
				// will be overwritten with the real initramfs files from dracut.
				dummyFile, err := os.Create(kernelBootFiles.initrdImagePath)
				if err != nil {
					return fmt.Errorf("failed to create (%s):\n%w", kernelBootFiles.initrdImagePath, err)
				}
				defer dummyFile.Close()

				_, err = dummyFile.WriteString(kernelBootFiles.initrdImagePath)
				if err != nil {
					return fmt.Errorf("failed to write to (%s):\n%w", kernelBootFiles.initrdImagePath, err)
				}
			}
		}

		targetEfiDir := filepath.Join(imageChroot.RootDir(), "boot/efi/EFI/BOOT")
		err = os.MkdirAll(targetEfiDir, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create destination efi directory (%s):\n%w", targetEfiDir, err)
		}

		targetShimPath := filepath.Join(targetEfiDir, filepath.Base(inputArtifactsStore.files.bootEfiPath))
		err = file.Copy(inputArtifactsStore.files.bootEfiPath, targetShimPath)
		if err != nil {
			return fmt.Errorf("failed to copy (%s) to (%s):\n%w", inputArtifactsStore.files.bootEfiPath, targetShimPath, err)
		}

		targetGrubPath := filepath.Join(targetEfiDir, filepath.Base(inputArtifactsStore.files.grubEfiPath))
		err = file.Copy(inputArtifactsStore.files.grubEfiPath, targetGrubPath)
		if err != nil {
			return fmt.Errorf("failed to copy (%s) to (%s):\n%w", inputArtifactsStore.files.grubEfiPath, targetGrubPath, err)
		}

		return err
	}

	// create the new raw disk image
	writeableChrootDir := "writeable-raw-image"
	_, err = CreateNewImage(targetOs, rawImageFile, diskConfig, fileSystemConfigs, buildDir, writeableChrootDir,
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
