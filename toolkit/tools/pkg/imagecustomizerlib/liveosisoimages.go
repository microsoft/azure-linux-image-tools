// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/cavaliercoder/go-cpio"
	"github.com/klauspost/pgzip"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/isogenerator"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safemount"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/targetos"
)

const (
	dracutConfig = `add_dracutmodules+=" dmsquash-live livenet selinux "
add_drivers+=" overlay "
hostonly="no"
`

	initScriptFileName = "init"
	initContent        = `mount -t proc proc /proc
/lib/systemd/systemd`

	// the total size of a collection of files is multiplied by the
	// expansionSafetyFactor to estimate a disk size sufficient to hold those
	// files.
	expansionSafetyFactor = 1.5

	// This folder is necessary to include in the initrd image so that the
	// emergency shell can work correctly with the keyboard.
	usrLibLocaleDir = "/usr/lib/locale"
)

func createInitrdImage(writeableRootfsDir, outputInitrdPath string) error {
	logger.Log.Debugf("Generating initrd (%s) from (%s)", outputInitrdPath, writeableRootfsDir)

	fstabFile := filepath.Join(writeableRootfsDir, "/etc/fstab")
	logger.Log.Debugf("Deleting fstab from %s", fstabFile)
	err := os.Remove(fstabFile)
	if err != nil {
		return fmt.Errorf("failed to delete fstab:\n%w", err)
	}

	initScriptPath := filepath.Join(writeableRootfsDir, initScriptFileName)
	err = os.WriteFile(initScriptPath, []byte(initContent), 0755)
	if err != nil {
		return fmt.Errorf("failed to create (%s):\n%w", initScriptPath, err)
	}

	outputFile, err := os.Create(outputInitrdPath)
	if err != nil {
		return fmt.Errorf("failed to create file (%s):\n%w", outputInitrdPath, err)
	}
	defer outputFile.Close()

	gzipWriter := pgzip.NewWriter(outputFile)
	defer gzipWriter.Close()

	cpioWriter := cpio.NewWriter(gzipWriter)
	defer func() {
		closeErr := cpioWriter.Close()
		if err != nil {
			err = closeErr
		}
	}()

	err = filepath.Walk(writeableRootfsDir, func(path string, info os.FileInfo, fileErr error) (err error) {
		if fileErr != nil {
			logger.Log.Warnf("File walk error on path (%s), error: %s", path, fileErr)
			return fileErr
		}
		err = addFileToArchive(writeableRootfsDir, path, info, cpioWriter)
		if err != nil {
			logger.Log.Warnf("Failed to add (%s), error: %s", path, err)
		}
		return nil
	})

	return nil
}

func addFileToArchive(inputDir, path string, info os.FileInfo, cpioWriter *cpio.Writer) (err error) {
	// Get the relative path of the file compared to the input directory.
	// The input directory should be considered the "root" of the cpio archive.
	relPath, err := filepath.Rel(inputDir, path)
	if err != nil {
		return
	}

	// logger.Log.Debugf("Adding to initrd: %s", relPath)

	// Symlinks need to be resolved to their target file to be added to the cpio archive.
	var link string
	if info.Mode()&os.ModeSymlink != 0 {
		link, err = os.Readlink(path)
		if err != nil {
			return
		}

		// logger.Log.Debugf("--> Adding link: (%s) -> (%s)", relPath, link)
	}

	// Convert the OS header into a CPIO header
	header, err := cpio.FileInfoHeader(info, link)
	if err != nil {
		return
	}

	// The default OS header will only have the filename as "Name".
	// Manually set the CPIO header's Name field to the relative path so it
	// is extracted to the correct directory.
	header.Name = relPath

	err = cpioWriter.WriteHeader(header)
	if err != nil {
		return
	}

	// Special files (unix sockets, directories, symlinks, ...) need to be handled differently
	// since a simple byte transfer of the file's content into the CPIO archive can't be achieved.
	if !info.Mode().IsRegular() {
		// For a symlink the reported size will be the size (in bytes) of the link's target.
		// Write this data into the archive.
		if info.Mode()&os.ModeSymlink != 0 {
			_, err = cpioWriter.Write([]byte(link))
		}

		// For all other special files, they will be of size 0 and only contain the header in the archive.
		return
	}

	// For regular files, open the actual file and copy its content into the archive.
	fileToAdd, err := os.Open(path)
	if err != nil {
		return
	}
	defer fileToAdd.Close()

	_, err = io.Copy(cpioWriter, fileToAdd)
	return
}

func extractFilesFromInitrdImage(initrdImagePath, outputDir string) error {
	f, err := os.Open(initrdImagePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	// Create pgzip reader
	gzr, err := pgzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("create pgzip reader: %w", err)
	}
	defer gzr.Close()

	// Create cpio reader
	cr := cpio.NewReader(gzr)

	for {
		hdr, err := cr.Next()
		if err == io.EOF {
			break // end of archive
		}
		if err != nil {
			return fmt.Errorf("read cpio header: %w", err)
		}

		path := filepath.Join(outputDir, hdr.Name)

		switch hdr.Mode & cpio.ModeType {
		case cpio.ModeDir:
			err := os.MkdirAll(path, os.FileMode(hdr.Mode&0777))
			if err != nil {
				return fmt.Errorf("create directory %s: %w", path, err)
			}
		case cpio.ModeRegular:
			err := os.MkdirAll(filepath.Dir(path), 0755)
			if err != nil {
				return fmt.Errorf("create parent dir for file %s: %w", path, err)
			}
			outFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode&cpio.ModePerm))
			if err != nil {
				return fmt.Errorf("create file %s: %w", path, err)
			}
			_, err = io.Copy(outFile, cr)
			outFile.Close()
			if err != nil {
				return fmt.Errorf("write file %s: %w", path, err)
			}
		default:
			fmt.Printf("Skipping unsupported type: %s\n", hdr.Name)
		}
	}

	return nil
}

func createMinimalInitrdImage(writeableRootfsDir, kernelVersion, outputInitrdPath string) error {
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
		filesStore.isoBootImagePath: "boot/grub2",
		filesStore.isoGrubCfgPath:   "boot/grub2",
		filesStore.vmlinuzPath:      "boot",
		filesStore.initrdImagePath:  "boot",
	}

	// Add optional squashfs file if it exists.
	if filesStore.squashfsImagePath != "" {
		exists, err := file.PathExists(filesStore.squashfsImagePath)
		if err != nil {
			return fmt.Errorf("failed to check if (%s) exists:\n%w", filesStore.squashfsImagePath, err)
		}
		if exists {
			artifactsToIsoMap[filesStore.squashfsImagePath] = "liveos"
		}
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

func createWriteableImageFromArtifacts(buildDir string, filesStore *IsoFilesStore, rawImageFile string) error {

	logger.Log.Infof("Creating full OS writeable image from ISO artifacts")

	// rootfs folder (mount squash fs)
	fullOSDir, err := os.MkdirTemp(buildDir, "tmp-full-os-mount-")
	if err != nil {
		return fmt.Errorf("failed to create temporary mount folder for squashfs:\n%w", err)
	}
	defer os.RemoveAll(fullOSDir)

	squashfsExists, err := file.PathExists(filesStore.squashfsImagePath)
	if err != nil {
		return fmt.Errorf("failed to check if the squash root file system image exists (%s):\n%w", filesStore.squashfsImagePath, err)
	}

	var squashfsLoopDevice *safeloopback.Loopback
	var squashfsMount *safemount.Mount

	if squashfsExists {
		squashfsLoopDevice, err = safeloopback.NewLoopback(filesStore.squashfsImagePath)
		if err != nil {
			return fmt.Errorf("failed to create loop device for (%s):\n%w", filesStore.squashfsImagePath, err)
		}
		defer squashfsLoopDevice.Close()

		squashfsMount, err = safemount.NewMount(squashfsLoopDevice.DevicePath(), fullOSDir,
			"squashfs" /*fstype*/, 0 /*flags*/, "" /*data*/, false /*makeAndDelete*/)
		if err != nil {
			return err
		}
		defer squashfsMount.Close()
	} else {
		err = extractFilesFromInitrdImage(filesStore.initrdImagePath, fullOSDir)
		if err != nil {
			return fmt.Errorf("failed to extract files from the initrd image (%s):\n%w", filesStore.initrdImagePath, err)
		}
	}

	// boot folder (from artifacts)
	artifactsBootDir := filepath.Join(filesStore.artifactsDir, "boot")

	imageContentList := []string{
		fullOSDir,
		filesStore.bootEfiPath,
		filesStore.grubEfiPath,
		artifactsBootDir}

	// estimate the new disk size
	safeDiskSizeMB, err := getDiskSizeEstimateInMBs(imageContentList, expansionSafetyFactor)
	if err != nil {
		return fmt.Errorf("failed to calculate the disk size of %s:\n%w", fullOSDir, err)
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

	targetOs, err := targetos.GetInstalledTargetOs(fullOSDir)
	if err != nil {
		return fmt.Errorf("failed to determine target OS of ISO squashfs:\n%w", err)
	}

	// populate the newly created disk image with content from the squash fs
	installOSFunc := func(imageChroot *safechroot.Chroot) error {
		// At the point when this copy will be executed, both the boot and the
		// root partitions will be mounted, and the files of /boot/efi will
		// land on the the boot partition, while the rest will be on the rootfs
		// partition.
		err := copyPartitionFiles(fullOSDir+"/.", imageChroot.RootDir())
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

		targetEfiDir := filepath.Join(imageChroot.RootDir(), "boot/efi/EFI/BOOT")
		err = os.MkdirAll(targetEfiDir, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create destination efi directory (%s):\n%w", targetEfiDir, err)
		}

		targetShimPath := filepath.Join(targetEfiDir, filepath.Base(filesStore.bootEfiPath))
		err = file.Copy(filesStore.bootEfiPath, targetShimPath)
		if err != nil {
			return fmt.Errorf("failed to copy (%s) to (%s):\n%w", filesStore.bootEfiPath, targetShimPath, err)
		}

		targetGrubPath := filepath.Join(targetEfiDir, filepath.Base(filesStore.grubEfiPath))
		err = file.Copy(filesStore.grubEfiPath, targetGrubPath)
		if err != nil {
			return fmt.Errorf("failed to copy (%s) to (%s):\n%w", filesStore.grubEfiPath, targetGrubPath, err)
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
