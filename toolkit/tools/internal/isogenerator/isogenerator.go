// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package isogenerator

import (
	"fmt"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
)

const (
	DefaultVolumeId                 = "CDROM"
	efiBootImgPathRelativeToIsoRoot = "boot/grub2/efiboot.img"
	initrdEFIBootDirectoryPath      = "boot/efi/EFI/BOOT"
)

func BuildIsoImage(stagingPath string, enableBiosBoot bool, isoOsFilesDirPath string, outputImagePath string) error {
	logger.Log.Infof("Creating ISO image: %s", outputImagePath)

	// For detailed parameter explanation see: https://linux.die.net/man/8/mkisofs.
	// Mkisofs requires all argument paths to be relative to the input directory.
	mkisofsArgs := []string{}

	mkisofsArgs = append(mkisofsArgs,
		// General mkisofs parameters.
		"-R", "-l", "-D", "-J", "-joliet-long", "-o", outputImagePath, "-V", DefaultVolumeId)

	if enableBiosBoot {
		mkisofsArgs = append(mkisofsArgs,
			// BIOS bootloader, params suggested by https://wiki.syslinux.org/wiki/index.php?title=ISOLINUX.
			"-b", filepath.Join(isoOsFilesDirPath, "isolinux.bin"),
			"-c", filepath.Join(isoOsFilesDirPath, "boot.cat"),
			"-no-emul-boot", "-boot-load-size", "4", "-boot-info-table")
	}

	mkisofsArgs = append(mkisofsArgs,
		// UEFI bootloader.
		"-eltorito-alt-boot", "-e", efiBootImgPathRelativeToIsoRoot, "-no-emul-boot",
		// Directory to convert to an ISO.
		stagingPath)

	// Note: mkisofs has a noisy stderr.
	err := shell.ExecuteLive(true /*squashErrors*/, "mkisofs", mkisofsArgs...)
	if err != nil {
		return fmt.Errorf("failed to generate ISO using mkisofs:\n%w", err)
	}

	return nil
}

func BuildIsoBootImage(buildDir string, sourceShimPath string, sourceGrubPath string, outputImagePath string) (err error) {
	logger.Log.Infof("Creating ISO bootloader image")

	const (
		blockSizeInBytes     = 1024 * 1024
		numberOfBlocksToCopy = 3
	)

	ddArgs := []string{
		"if=/dev/zero",                                // Zero device to read a stream of zeroed bytes from.
		fmt.Sprintf("of=%s", outputImagePath),         // Output file.
		fmt.Sprintf("bs=%d", blockSizeInBytes),        // Size of one copied block. Used together with "count".
		fmt.Sprintf("count=%d", numberOfBlocksToCopy), // Number of blocks to copy to the output file.
	}
	logger.Log.Debugf("Creating an empty '%s' file of %d bytes.", outputImagePath, blockSizeInBytes*numberOfBlocksToCopy)

	// Note: dd has a noisy stderr.
	err = shell.ExecuteLive(true /*squashErrors*/, "dd", ddArgs...)
	if err != nil {
		return err
	}

	logger.Log.Debugf("Formatting '%s' as an MS-DOS filesystem.", outputImagePath)
	err = shell.ExecuteLive(true /*squashErrors*/, "mkdosfs", outputImagePath)
	if err != nil {
		return err
	}

	efiBootImgTempMountDir := filepath.Join(buildDir, "efiboot_temp")

	logger.Log.Debugf("Mounting '%s' to '%s' to copy EFI modules required to boot grub2.", outputImagePath,
		efiBootImgTempMountDir)
	loopback, err := safeloopback.NewLoopback(outputImagePath)
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
	bootDirPath := filepath.Join(efiBootImgTempMountDir, "EFI", "BOOT")

	targetShimPath := filepath.Join(bootDirPath, filepath.Base(sourceShimPath))
	err = file.Copy(sourceShimPath, targetShimPath)
	if err != nil {
		return err
	}

	targetGrubPath := filepath.Join(bootDirPath, filepath.Base(sourceGrubPath))
	err = file.Copy(sourceGrubPath, targetGrubPath)
	if err != nil {
		return err
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
func ApplyRufusWorkaround(sourceShimPath, sourceGrubPath, stagingPath string) (err error) {
	const buildDirBootEFIDirectoryPath = "efi/boot"

	targetShimPath := filepath.Join(stagingPath, buildDirBootEFIDirectoryPath, filepath.Base(sourceShimPath))
	err = file.Copy(sourceShimPath, targetShimPath)
	if err != nil {
		return fmt.Errorf("failed to copy (%s) to (%s):\n%w", sourceShimPath, targetShimPath, err)
	}

	targetGrubPath := filepath.Join(stagingPath, buildDirBootEFIDirectoryPath, filepath.Base(sourceGrubPath))
	err = file.Copy(sourceGrubPath, targetGrubPath)
	if err != nil {
		return fmt.Errorf("failed to copy (%s) to (%s):\n%w", sourceGrubPath, targetGrubPath, err)
	}

	return nil
}
