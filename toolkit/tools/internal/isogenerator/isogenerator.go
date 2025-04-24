// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package isogenerator

import (
	"fmt"
	"path/filepath"

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

func BuildIsoImage(stagingPath string, outputImagePath string) error {
	logger.Log.Infof("-- Generating ISO image (%s) using (%s).", outputImagePath, stagingPath)

	// For detailed parameter explanation see: https://linux.die.net/man/8/mkisofs.
	// Mkisofs requires all argument paths to be relative to the input directory.
	mkisofsArgs := []string{}

	mkisofsArgs = append(mkisofsArgs,
		// General mkisofs parameters.
		"-R", "-l", "-D", "-J", "-joliet-long", "-o", outputImagePath, "-V", DefaultVolumeId,
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

func BuildIsoBootImage(buildDir string, shimPath string, grubPath string, outputImagePath string) (err error) {
	const (
		blockSizeInBytes     = 1024 * 1024
		numberOfBlocksToCopy = 3
	)

	logger.Log.Info("Preparing ISO's bootloaders.")

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
	shimFileName := filepath.Base(shimPath)
	bootDirPath := filepath.Join(efiBootImgTempMountDir, "EFI", "BOOT")
	targetShimPath := filepath.Join(bootDirPath, shimFileName)
	err = file.Copy(shimPath, targetShimPath)
	if err != nil {
		return err
	}

	grubFileName := filepath.Base(grubPath)
	targetGrubPath := filepath.Join(bootDirPath, grubFileName)
	err = file.Copy(grubPath, targetGrubPath)
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
