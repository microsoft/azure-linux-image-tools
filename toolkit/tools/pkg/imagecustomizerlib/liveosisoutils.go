// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"runtime"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"golang.org/x/sys/unix"
)

const (
	osEspDir           = "/boot/efi"
	espBootloaderDir   = "EFI/BOOT"
	osEspBootloaderDir = osEspDir + "/" + espBootloaderDir
	isoBootloaderDir   = "/efi/boot"
	ukiEfiStubDir      = "/usr/lib/systemd/boot/efi"

	bootx64Binary  = "bootx64.efi"
	bootAA64Binary = "bootaa64.efi"

	grubx64Binary  = "grubx64.efi"
	grubAA64Binary = "grubaa64.efi"

	grubx64NoPrefixBinary  = "grubx64-noprefix.efi"
	grubAA64NoPrefixBinary = "grubaa64-noprefix.efi"

	ukiEfiStubx64Binary  = "linuxx64.efi.stub"
	ukiEfiStubAA64Binary = "linuxaa64.efi.stub"

	ukiAddonStubx64Binary  = "addonx64.efi.stub"
	ukiAddonStubAA64Binary = "addonaa64.efi.stub"

	grubCfgDir     = "/boot/grub2"
	isoGrubCfg     = "grub.cfg"
	isoGrubCfgPath = grubCfgDir + "/" + isoGrubCfg

	pxeGrubCfg = "grub-pxe.cfg"

	initrdImage = "initrd.img"

	// In vhd(x)/qcow/iso images, the kernel is named 'vmlinuz-<version>'.
	vmLinuzPrefix    = "vmlinuz-"
	initramfsPrefix  = "initramfs-"  // AZL3, Fedora, etc.
	initrdPrefix     = "initrd.img-" // AZL2, Ubuntu, etc.
	isoKernelDir     = "/boot"
	isoInitrdPath    = "/boot/" + initrdImage
	isoBootImagePath = "/boot/grub2/efiboot.img"
)

type BootFilesArchConfig struct {
	bootBinary                  string
	grubBinary                  string
	grubNoPrefixBinary          string
	espBootBinaryPath           string
	espGrubBinaryPath           string
	osEspBootBinaryPath         string
	osEspGrubBinaryPath         string
	osEspGrubNoPrefixBinaryPath string
	isoBootBinaryPath           string
	isoGrubBinaryPath           string
	ukiEfiStubBinary            string
	ukiEfiStubBinaryPath        string
	ukiAddonStubBinary          string
	ukiAddonStubBinaryPath      string
}

var bootloaderFilesConfigAzureLinux = map[string]BootFilesArchConfig{
	"amd64": {
		bootBinary:                  bootx64Binary,
		grubBinary:                  grubx64Binary,
		grubNoPrefixBinary:          grubx64NoPrefixBinary,
		espBootBinaryPath:           espBootloaderDir + "/" + bootx64Binary,
		espGrubBinaryPath:           espBootloaderDir + "/" + grubx64Binary,
		osEspBootBinaryPath:         osEspBootloaderDir + "/" + bootx64Binary,
		osEspGrubBinaryPath:         osEspBootloaderDir + "/" + grubx64Binary,
		osEspGrubNoPrefixBinaryPath: osEspBootloaderDir + "/" + grubx64NoPrefixBinary,
		isoBootBinaryPath:           isoBootloaderDir + "/" + bootx64Binary,
		isoGrubBinaryPath:           isoBootloaderDir + "/" + grubx64Binary,
		ukiEfiStubBinary:            ukiEfiStubx64Binary,
		ukiEfiStubBinaryPath:        ukiEfiStubDir + "/" + ukiEfiStubx64Binary,
		ukiAddonStubBinary:          ukiAddonStubx64Binary,
		ukiAddonStubBinaryPath:      ukiEfiStubDir + "/" + ukiAddonStubx64Binary,
	},
	"arm64": {
		bootBinary:                  bootAA64Binary,
		grubBinary:                  grubAA64Binary,
		grubNoPrefixBinary:          grubAA64NoPrefixBinary,
		espBootBinaryPath:           espBootloaderDir + "/" + bootAA64Binary,
		espGrubBinaryPath:           espBootloaderDir + "/" + grubAA64Binary,
		osEspBootBinaryPath:         osEspBootloaderDir + "/" + bootAA64Binary,
		osEspGrubBinaryPath:         osEspBootloaderDir + "/" + grubAA64Binary,
		osEspGrubNoPrefixBinaryPath: osEspBootloaderDir + "/" + grubAA64NoPrefixBinary,
		isoBootBinaryPath:           isoBootloaderDir + "/" + bootAA64Binary,
		isoGrubBinaryPath:           isoBootloaderDir + "/" + grubAA64Binary,
		ukiEfiStubBinary:            ukiEfiStubAA64Binary,
		ukiEfiStubBinaryPath:        ukiEfiStubDir + "/" + ukiEfiStubAA64Binary,
		ukiAddonStubBinary:          ukiAddonStubAA64Binary,
		ukiAddonStubBinaryPath:      ukiEfiStubDir + "/" + ukiAddonStubAA64Binary,
	},
}

// bootArchConfigFromMap looks up the current runtime architecture in the provided per-arch boot files config map.
func bootArchConfigFromMap(configByArch map[string]BootFilesArchConfig) (BootFilesArchConfig, error) {
	arch := runtime.GOARCH
	switch arch {
	case "amd64", "arm64":
		return configByArch[arch], nil
	default:
		return BootFilesArchConfig{}, fmt.Errorf("unsupported architecture: %s", arch)
	}
}

func extractIsoImageContents(buildDir string, isoImageFile string, isoExpansionFolder string) (err error) {
	mountDir, err := os.MkdirTemp(buildDir, "tmp-iso-mount-")
	if err != nil {
		return fmt.Errorf("failed to create temporary mount folder for iso:\n%w", err)
	}
	defer os.RemoveAll(mountDir)

	isoImageLoopDevice, err := safeloopback.NewLoopback(isoImageFile)
	if err != nil {
		return fmt.Errorf("failed to create loop device for (%s):\n%w", isoImageFile, err)
	}
	defer isoImageLoopDevice.Close()

	isoImageMount, err := safemount.NewMount(isoImageLoopDevice.DevicePath(), mountDir,
		"iso9660" /*fstype*/, unix.MS_RDONLY /*flags*/, "" /*data*/, false /*makeAndDelete*/)
	if err != nil {
		return err
	}
	defer isoImageMount.Close()

	err = os.MkdirAll(isoExpansionFolder, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create folder (%s):\n%w", isoExpansionFolder, err)
	}

	err = copyPartitionFiles(mountDir+"/.", isoExpansionFolder)
	if err != nil {
		return fmt.Errorf("failed to copy iso image contents to a writeable folder (%s):\n%w", isoExpansionFolder, err)
	}

	err = isoImageMount.CleanClose()
	if err != nil {
		return err
	}

	err = isoImageLoopDevice.CleanClose()
	if err != nil {
		return err
	}

	return nil
}
