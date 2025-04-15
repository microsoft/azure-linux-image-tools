// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package isogenerator

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cavaliercoder/go-cpio"
	"github.com/klauspost/pgzip"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safemount"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
)

const (
	DefaultVolumeId                 = "CDROM"
	efiBootImgPathRelativeToIsoRoot = "boot/grub2/efiboot.img"
	initrdEFIBootDirectoryPath      = "boot/efi/EFI/BOOT"
)

type IsoGenConfig struct {
	// Directory where temporary files can be stored.
	BuildDirPath string
	// Directory where to stage ISO files.
	// If the directory exists, any existing files will be included in the ISO.
	StagingDirPath string
	// Enable legacy boot.
	// Note: This isn't useful unless some additional assets are included in 'StagingDirPath'.
	EnableBiosBoot bool
	// The path where the ISO file will be written.
	OutputFilePath string
	// The directory in the ISO where the following files will be written to:
	// - initrd.img
	// - vmlinuz
	// - isolinux.bin (for BIOS boot)
	// - boot.cat (for BIOS boot)
	IsoOsFilesDirPath string
	// The path of the initrd file to use in the ISO.
	InitrdPath string
}

type isoGenInfo struct {
	Config         IsoGenConfig
	EfiBootImgPath string
}

func GenerateIso(config IsoGenConfig) error {
	err := os.MkdirAll(config.StagingDirPath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create ISO staging directory (%s):\n%w", config.StagingDirPath, err)
	}

	efiBootImgPath := filepath.Join(config.StagingDirPath, efiBootImgPathRelativeToIsoRoot)

	info := isoGenInfo{
		Config:         config,
		EfiBootImgPath: efiBootImgPath,
	}

	err = prepareIsoBootLoaderFilesAndFolders(info)
	if err != nil {
		return err
	}

	err = buildIsoImage(info)
	if err != nil {
		return err
	}

	return nil
}

func buildIsoImage(info isoGenInfo) error {
	logger.Log.Infof("Generating ISO image under '%s'.", info.Config.OutputFilePath)

	// For detailed parameter explanation see: https://linux.die.net/man/8/mkisofs.
	// Mkisofs requires all argument paths to be relative to the input directory.
	mkisofsArgs := []string{}

	mkisofsArgs = append(mkisofsArgs,
		// General mkisofs parameters.
		"-R", "-l", "-D", "-J", "-joliet-long", "-o", info.Config.OutputFilePath, "-V", DefaultVolumeId)

	if info.Config.EnableBiosBoot {
		mkisofsArgs = append(mkisofsArgs,
			// BIOS bootloader, params suggested by https://wiki.syslinux.org/wiki/index.php?title=ISOLINUX.
			"-b", filepath.Join(info.Config.IsoOsFilesDirPath, "isolinux.bin"),
			"-c", filepath.Join(info.Config.IsoOsFilesDirPath, "boot.cat"),
			"-no-emul-boot", "-boot-load-size", "4", "-boot-info-table")
	}

	mkisofsArgs = append(mkisofsArgs,
		// UEFI bootloader.
		"-eltorito-alt-boot", "-e", efiBootImgPathRelativeToIsoRoot, "-no-emul-boot",

		// Directory to convert to an ISO.
		info.Config.StagingDirPath)

	// Note: mkisofs has a noisy stderr.
	err := shell.ExecuteLive(true /*squashErrors*/, "mkisofs", mkisofsArgs...)
	if err != nil {
		return fmt.Errorf("failed to generate ISO using mkisofs:\n%w", err)
	}

	return nil
}

// prepareIsoBootLoaderFilesAndFolders copies the files required by the ISO's bootloader
func prepareIsoBootLoaderFilesAndFolders(info isoGenInfo) (err error) {
	err = setUpIsoGrub2Bootloader(info)
	if err != nil {
		return err
	}

	err = createVmlinuzImage(info)
	if err != nil {
		return err
	}

	err = copyInitrd(info)
	if err != nil {
		return err
	}

	return nil
}

// copyInitrd copies a pre-built initrd into the isolinux folder.
func copyInitrd(info isoGenInfo) error {
	initrdDestinationPath := filepath.Join(info.Config.StagingDirPath, info.Config.IsoOsFilesDirPath, "initrd.img")

	logger.Log.Debugf("Copying initrd from '%s'.", info.Config.InitrdPath)

	return file.Copy(info.Config.InitrdPath, initrdDestinationPath)
}

func setUpIsoGrub2Bootloader(info isoGenInfo) (err error) {
	const (
		blockSizeInBytes     = 1024 * 1024
		numberOfBlocksToCopy = 3
	)

	logger.Log.Info("Preparing ISO's bootloaders.")

	ddArgs := []string{
		"if=/dev/zero", // Zero device to read a stream of zeroed bytes from.
		fmt.Sprintf("of=%s", info.EfiBootImgPath),     // Output file.
		fmt.Sprintf("bs=%d", blockSizeInBytes),        // Size of one copied block. Used together with "count".
		fmt.Sprintf("count=%d", numberOfBlocksToCopy), // Number of blocks to copy to the output file.
	}
	logger.Log.Debugf("Creating an empty '%s' file of %d bytes.", info.EfiBootImgPath, blockSizeInBytes*numberOfBlocksToCopy)

	// Note: dd has a noisy stderr.
	err = shell.ExecuteLive(true /*squashErrors*/, "dd", ddArgs...)
	if err != nil {
		return err
	}

	logger.Log.Debugf("Formatting '%s' as an MS-DOS filesystem.", info.EfiBootImgPath)
	err = shell.ExecuteLive(true /*squashErrors*/, "mkdosfs", info.EfiBootImgPath)
	if err != nil {
		return err
	}

	efiBootImgTempMountDir := filepath.Join(info.Config.BuildDirPath, "efiboot_temp")

	logger.Log.Debugf("Mounting '%s' to '%s' to copy EFI modules required to boot grub2.", info.EfiBootImgPath,
		efiBootImgTempMountDir)
	loopback, err := safeloopback.NewLoopback(info.EfiBootImgPath)
	if err != nil {
		return fmt.Errorf("failed to connect efiboot.img:\n%w", err)
	}
	defer loopback.Close()

	mount, err := safemount.NewMount(loopback.DevicePath(), efiBootImgTempMountDir, "vfat", 0, "",
		true /*makeAndDeleteDir*/)
	if err != nil {
		return fmt.Errorf("failed to mount efiboot.img:\n%w", err)
	}
	defer mount.Close()

	logger.Log.Debug("Copying EFI modules into efiboot.img.")
	// Copy Shim (boot<arch>64.efi) and grub2 (grub<arch>64.efi)
	if runtime.GOARCH == "arm64" {
		err = copyShimFromInitrd(info, efiBootImgTempMountDir, "bootaa64.efi", "grubaa64.efi")
		if err != nil {
			return err
		}
	} else {
		err = copyShimFromInitrd(info, efiBootImgTempMountDir, "bootx64.efi", "grubx64.efi")
		if err != nil {
			return err
		}
	}

	err = mount.CleanClose()
	if err != nil {
		return fmt.Errorf("failed to unmount efiboot.img:\n%w", err)
	}

	err = loopback.CleanClose()
	if err != nil {
		return fmt.Errorf("failed to disconnect efiboot.img:\n%w", err)
	}

	return nil
}

func copyShimFromInitrd(info isoGenInfo, efiBootImgTempMountDir, bootBootloaderFile, grubBootloaderFile string,
) (err error) {
	bootDirPath := filepath.Join(efiBootImgTempMountDir, "EFI", "BOOT")

	initrdBootBootloaderFilePath := filepath.Join(initrdEFIBootDirectoryPath, bootBootloaderFile)
	buildDirBootEFIFilePath := filepath.Join(bootDirPath, bootBootloaderFile)
	err = extractFromInitrdAndCopy(info, initrdBootBootloaderFilePath, buildDirBootEFIFilePath)
	if err != nil {
		return err
	}

	initrdGrubBootloaderFilePath := filepath.Join(initrdEFIBootDirectoryPath, grubBootloaderFile)
	buildDirGrubEFIFilePath := filepath.Join(bootDirPath, grubBootloaderFile)
	err = extractFromInitrdAndCopy(info, initrdGrubBootloaderFilePath, buildDirGrubEFIFilePath)
	if err != nil {
		return err
	}

	err = applyRufusWorkaround(info, bootBootloaderFile, grubBootloaderFile)
	if err != nil {
		return err
	}

	return nil
}

// Rufus ISO-to-USB converter has a limitation where it will only copy the boot<arch>64.efi binary from a given efi*.img
// archive into the standard UEFI EFI/BOOT folder instead of extracting the whole archive as per the El Torito ISO
// specification.
//
// Most distros (including ours) use a 2 stage bootloader flow (shim->grub->kernel). Since the Rufus limitation only
// copies the 1st stage to EFI/BOOT/boot<arch>64.efi, it cannot find the 2nd stage bootloader (grub<arch>64.efi) which should
// be in the same directory: EFI/BOOT/grub<arch>64.efi. This causes the USB installation to fail to boot.
//
// Rufus prioritizes the presence of an EFI folder on the ISO disk over extraction of the efi*.img archive.
// So to workaround the limitation, create an EFI folder and make a duplicate copy of the bootloader files
// in EFI/Boot so Rufus doesn't attempt to extract the efi*.img in the first place.
func applyRufusWorkaround(info isoGenInfo, bootBootloaderFile string, grubBootloaderFile string) (err error) {
	const buildDirBootEFIDirectoryPath = "efi/boot"

	initrdBootloaderFilePath := filepath.Join(initrdEFIBootDirectoryPath, bootBootloaderFile)
	buildDirBootEFIUsbFilePath := filepath.Join(info.Config.StagingDirPath, buildDirBootEFIDirectoryPath, bootBootloaderFile)
	err = extractFromInitrdAndCopy(info, initrdBootloaderFilePath, buildDirBootEFIUsbFilePath)
	if err != nil {
		return err
	}

	initrdGrubEFIFilePath := filepath.Join(initrdEFIBootDirectoryPath, grubBootloaderFile)
	buildDirGrubEFIUsbFilePath := filepath.Join(info.Config.StagingDirPath, buildDirBootEFIDirectoryPath, grubBootloaderFile)
	err = extractFromInitrdAndCopy(info, initrdGrubEFIFilePath, buildDirGrubEFIUsbFilePath)
	if err != nil {
		return err
	}

	return nil
}

// createVmlinuzImage builds the 'vmlinuz' file containing the Linux kernel
// ran by the ISO bootloader.
func createVmlinuzImage(info isoGenInfo) error {
	const bootKernelFile = "boot/vmlinuz"

	vmlinuzFilePath := filepath.Join(info.Config.StagingDirPath, info.Config.IsoOsFilesDirPath, "vmlinuz")

	// In order to select the correct kernel for isolinux, open the initrd archive
	// and extract the vmlinuz file in it. An initrd is a gzip of a cpio archive.
	//
	return extractFromInitrdAndCopy(info, bootKernelFile, vmlinuzFilePath)
}

func extractFromInitrdAndCopy(info isoGenInfo, srcFileName, destFilePath string) (err error) {
	// Setup a series of io readers: initrd file -> parallelized gzip -> cpio

	logger.Log.Debugf("Searching for (%s) in initrd (%s) and copying to (%s)", srcFileName, info.Config.InitrdPath, destFilePath)

	initrdFile, err := os.Open(info.Config.InitrdPath)
	if err != nil {
		return err
	}
	defer initrdFile.Close()

	gzipReader, err := pgzip.NewReader(initrdFile)
	if err != nil {
		return err
	}
	cpioReader := cpio.NewReader(gzipReader)

	for {
		// Search through the headers until the source file is found
		var hdr *cpio.Header
		hdr, err = cpioReader.Next()
		if err == io.EOF {
			return fmt.Errorf("did not find (%s) in initrd (%s)", srcFileName, info.Config.InitrdPath)
		}
		if err != nil {
			return err
		}

		if strings.HasPrefix(hdr.Name, srcFileName) {
			logger.Log.Debugf("Found source file (%s) in initrd", srcFileName)
			// Source file found, copy it to destination
			err = os.MkdirAll(filepath.Dir(destFilePath), os.ModePerm)
			if err != nil {
				return err
			}

			dstFile, err := os.Create(destFilePath)
			if err != nil {
				return err
			}
			defer dstFile.Close()

			logger.Log.Debugf("Copying (%s) to (%s)", srcFileName, destFilePath)
			_, err = io.Copy(dstFile, cpioReader)
			if err != nil {
				return err
			}
			break
		}
	}
	return nil
}
