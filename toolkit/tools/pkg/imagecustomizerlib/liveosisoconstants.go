// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

const (
	osEspBootloaderDir = "/boot/efi/EFI/BOOT"
	isoBootloaderDir   = "/efi/boot"

	bootx64Binary  = "bootx64.efi"
	bootAA64Binary = "bootaa64.efi"

	grubx64Binary  = "grubx64.efi"
	grubAA64Binary = "grubaa64.efi"

	grubx64NoPrefixBinary  = "grubx64-noprefix.efi"
	grubAA64NoPrefixBinary = "grubaa64-noprefix.efi"

	systemdBootx64Binary  = "systemd-bootx64.efi"
	systemdBootAA64Binary = "systemd-bootaa64.efi"

	grubCfgDir     = "/boot/grub2"
	isoGrubCfg     = "grub.cfg"
	isoGrubCfgPath = grubCfgDir + "/" + isoGrubCfg

	pxeGrubCfg = "grub-pxe.cfg"

	isoBootDir  = "boot"
	initrdImage = "initrd.img"
	// In vhd(x)/qcow images, the kernel is named 'vmlinuz-<version>'.
	// In the ISO image, the kernel is named 'vmlinuz'.
	vmLinuzPrefix     = "vmlinuz"
	isoInitrdPath     = "/boot/" + initrdImage
	isoKernelPath     = "/boot/vmlinuz"
	isoBootloadersDir = "/efi/boot"
	isoBootImagePath  = "/boot/grub2/efiboot.img"

	// This folder is necessary to include in the initrd image so that the
	// emergency shell can work correctly with the keyboard.
	usrLibLocaleDir = "/usr/lib/locale"

	liveOSDir       = "liveos"
	liveOSImage     = "rootfs.img"
	liveOSImagePath = "/" + liveOSDir + "/" + liveOSImage
)
