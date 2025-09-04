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
	// The path of the initrd file to use in the ISO.
	InitrdPath string
	// Enable legacy boot.
	// Note: This isn't useful unless some additional assets are included in 'StagingDirPath'.
	EnableBiosBoot bool
	// The directory in the ISO where the following files will be written to:
	// - initrd.img
	// - vmlinuz
	// - isolinux.bin (for BIOS boot)
	// - boot.cat (for BIOS boot)
	IsoOsFilesDirPath string
	// The path where the ISO file will be written.
	OutputFilePath string
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

	err = BuildIsoImage(config.StagingDirPath, config.EnableBiosBoot, config.IsoOsFilesDirPath,
		config.OutputFilePath)
	if err != nil {
		return err
	}

	return nil
}

func BuildIsoImage(stagingPath string, enableBiosBoot bool, isoOsFilesDirPath string, outputImagePath string) error {
	logger.Log.Infof("Creating ISO image: %s", outputImagePath)

	// For detailed parameter explanation see: https://linux.die.net/man/8/mkisofs.
	// Mkisofs requires all argument paths to be relative to the input directory.

	// The reason we are using "xorriso -as mkisofs" is because the parameters that
	// enable the creation of images with files > 4GiB are no exposed in the xorriso
	// binary native parameters - however, they are exposed through the "-as mkisofs".
	// These parameters are: -iso-level, -udf, and -allow-limited-size.

	mkisofsArgs := []string{}

	mkisofsArgs = append(mkisofsArgs,
		"-as", "mkisofs",
		// General mkisofs parameters.
		"-R", "-l", "-D",
		"-iso-level", "3", // "-udf", "-allow-limited-size", // allow files larger than 4GB.
		"-J", "-joliet-long",
		"-o", outputImagePath, "-V", DefaultVolumeId)

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
	err := shell.ExecuteLive(true /*squashErrors*/, "xorriso", mkisofsArgs...)
	if err != nil {
		return fmt.Errorf("failed to generate ISO using xorriso:\n%w", err)
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

func setUpIsoGrub2Bootloader(info isoGenInfo) (err error) {

	extractedShimDir, err := os.MkdirTemp(info.Config.BuildDirPath, "extracted-shim")
	if err != nil {
		return fmt.Errorf("failed to create temporary folder for extracting the shim:\n%w", err)
	}
	defer os.RemoveAll(extractedShimDir)

	shimFileName := ""
	grubFileName := ""
	switch runtime.GOARCH {
	case "arm64":
		shimFileName = "bootaa64.efi"
		grubFileName = "grubaa64.efi"
	case "amd64":
		shimFileName = "bootx64.efi"
		grubFileName = "grubx64.efi"
	default:
		return fmt.Errorf("failed to determine shim/grub efi file names. Unsupported host architecture (%s)", runtime.GOARCH)
	}

	// Extract the shim/grub binaries.
	shimPath, grubPath, err := extractShimFromInitrd(info.Config.InitrdPath, extractedShimDir, shimFileName, grubFileName)
	if err != nil {
		return err
	}

	// Pack the extracted shim/grub binaries into the iso boot image.
	err = BuildIsoBootImage(info.Config.BuildDirPath, shimPath, grubPath, info.EfiBootImgPath)
	if err != nil {
		return nil
	}

	// Copy the extracted shim/grub binaries to the Rufus workaround folder.
	err = ApplyRufusWorkaround(shimPath, grubPath, info.Config.StagingDirPath)
	if err != nil {
		return err
	}

	return nil
}

func extractShimFromInitrd(initrdPath, outputDir, bootBootloaderFile, grubBootloaderFile string,
) (buildDirBootEFIFilePath string, buildDirGrubEFIFilePath string, err error) {

	initrdBootBootloaderFilePath := filepath.Join(initrdEFIBootDirectoryPath, bootBootloaderFile)
	buildDirBootEFIFilePath = filepath.Join(outputDir, bootBootloaderFile)
	err = extractFromInitrdAndCopy(initrdPath, initrdBootBootloaderFilePath, buildDirBootEFIFilePath)
	if err != nil {
		return "", "", err
	}

	initrdGrubBootloaderFilePath := filepath.Join(initrdEFIBootDirectoryPath, grubBootloaderFile)
	buildDirGrubEFIFilePath = filepath.Join(outputDir, grubBootloaderFile)
	err = extractFromInitrdAndCopy(initrdPath, initrdGrubBootloaderFilePath, buildDirGrubEFIFilePath)
	if err != nil {
		return "", "", err
	}

	return buildDirBootEFIFilePath, buildDirGrubEFIFilePath, nil
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

// createVmlinuzImage builds the 'vmlinuz' file containing the Linux kernel
// ran by the ISO bootloader.
func createVmlinuzImage(info isoGenInfo) error {
	const bootKernelFile = "boot/vmlinuz"

	vmlinuzFilePath := filepath.Join(info.Config.StagingDirPath, info.Config.IsoOsFilesDirPath, "vmlinuz")

	// In order to select the correct kernel for isolinux, open the initrd archive
	// and extract the vmlinuz file in it. An initrd is a gzip of a cpio archive.
	//
	return extractFromInitrdAndCopy(info.Config.InitrdPath, bootKernelFile, vmlinuzFilePath)
}

func extractFromInitrdAndCopy(initrdPath, srcFileName, destFilePath string) (err error) {
	// Setup a series of io readers: initrd file -> parallelized gzip -> cpio

	logger.Log.Debugf("Searching for (%s) in initrd (%s) and copying to (%s)", srcFileName, initrdPath, destFilePath)

	initrdFile, err := os.Open(initrdPath)
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
			return fmt.Errorf("did not find (%s) in initrd (%s)", srcFileName, initrdPath)
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
