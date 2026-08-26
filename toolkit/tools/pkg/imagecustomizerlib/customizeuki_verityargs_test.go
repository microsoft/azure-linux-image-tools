// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAppendKernelArgsToUkiCmdlineFilePreservesExistingAddons verifies that appending fresh
// verity args via appendKernelArgsToUkiCmdlineFile (the internal /usr dm-verity root-hash refresh
// path, distinct from user kernelCommandLine customization) does not discard a kernel's
// ExistingAddons -- the same class of bug already fixed for bootcustomizer.go's
// appendToUkiCmdlineFile/updateUkiCmdlineFile (see bootcustomizer_ukicmdline_test.go), but present
// in this sibling function too until now.
func TestAppendKernelArgsToUkiCmdlineFilePreservesExistingAddons(t *testing.T) {
	testTempDir := t.TempDir()
	kernelInfoDir := filepath.Join(testTempDir, UkiBuildDir)
	err := os.MkdirAll(kernelInfoDir, os.ModePerm)
	assert.NoError(t, err)
	kernelInfoPath := filepath.Join(kernelInfoDir, UkiKernelInfoJson)

	existingAddons := map[string]string{
		ukiMainCmdlineAddonKey: "root=LABEL=ROOT",
		"oem.addon.efi":        "flatcar.oem.id=azure",
		"verity.addon.efi": "systemd.verity_usr_data=PARTUUID=aaa systemd.verity_usr_hash=PARTUUID=bbb " +
			"systemd.verity_usr_options=panic-on-corruption usrhash=oldhash",
	}
	initialKernelInfo := map[string]UkiKernelInfo{
		"vmlinuz-6.6.92.2-2.azl3": {
			Cmdline: "root=LABEL=ROOT flatcar.oem.id=azure systemd.verity_usr_data=PARTUUID=aaa " +
				"systemd.verity_usr_hash=PARTUUID=bbb systemd.verity_usr_options=panic-on-corruption usrhash=oldhash",
			Initramfs:      "initrd-6.6.92.2-2.azl3",
			ExistingAddons: existingAddons,
		},
	}
	err = writeUkiKernelInfoFile(kernelInfoPath, initialKernelInfo)
	assert.NoError(t, err)

	newArgs := []string{
		"systemd.verity_usr_data=PARTUUID=aaa",
		"systemd.verity_usr_hash=PARTUUID=bbb",
		"systemd.verity_usr_options=panic-on-corruption",
		"usrhash=newhash",
	}
	err = appendKernelArgsToUkiCmdlineFile(testTempDir, newArgs)
	assert.NoError(t, err)

	updatedKernelInfo, err := readUkiKernelInfoFile(filepath.Join(testTempDir, UkiBuildDir, UkiKernelInfoJson))
	assert.NoError(t, err)

	info := updatedKernelInfo["vmlinuz-6.6.92.2-2.azl3"]
	assert.Equal(t,
		"root=LABEL=ROOT flatcar.oem.id=azure systemd.verity_usr_data=PARTUUID=aaa "+
			"systemd.verity_usr_hash=PARTUUID=bbb systemd.verity_usr_options=panic-on-corruption usrhash=newhash",
		info.Cmdline)
	assert.Equal(t, "initrd-6.6.92.2-2.azl3", info.Initramfs)

	// ExistingAddons must be preserved (not dropped), and the file that previously carried the
	// stale verity args must be refreshed in place with the new ones, so it stays consistent with
	// Cmdline -- otherwise a DistroHandler mirroring ExistingAddons' structure (e.g. ACL's
	// aclGetUkiAddonSpecsPreserving) would see the refresh as a rejected verity-arg change.
	assert.NotNil(t, info.ExistingAddons)
	assert.Equal(t, "root=LABEL=ROOT", info.ExistingAddons[ukiMainCmdlineAddonKey])
	assert.Equal(t, "flatcar.oem.id=azure", info.ExistingAddons["oem.addon.efi"])
	assert.Equal(t,
		"systemd.verity_usr_data=PARTUUID=aaa systemd.verity_usr_hash=PARTUUID=bbb "+
			"systemd.verity_usr_options=panic-on-corruption usrhash=newhash",
		info.ExistingAddons["verity.addon.efi"])
}

// TestAppendKernelArgsToUkiCmdlineFileNilExistingAddons verifies appendKernelArgsToUkiCmdlineFile
// tolerates a kernel with no prior ExistingAddons at all (e.g. a brand-new kernel), leaving it nil
// rather than panicking or fabricating a structure.
func TestAppendKernelArgsToUkiCmdlineFileNilExistingAddons(t *testing.T) {
	testTempDir := t.TempDir()
	kernelInfoDir := filepath.Join(testTempDir, UkiBuildDir)
	err := os.MkdirAll(kernelInfoDir, os.ModePerm)
	assert.NoError(t, err)
	kernelInfoPath := filepath.Join(kernelInfoDir, UkiKernelInfoJson)

	initialKernelInfo := map[string]UkiKernelInfo{
		"vmlinuz-6.6.92.2-2.azl3": {
			Cmdline:   "root=LABEL=ROOT",
			Initramfs: "initrd-6.6.92.2-2.azl3",
		},
	}
	err = writeUkiKernelInfoFile(kernelInfoPath, initialKernelInfo)
	assert.NoError(t, err)

	err = appendKernelArgsToUkiCmdlineFile(testTempDir, []string{"usrhash=newhash"})
	assert.NoError(t, err)

	updatedKernelInfo, err := readUkiKernelInfoFile(filepath.Join(testTempDir, UkiBuildDir, UkiKernelInfoJson))
	assert.NoError(t, err)

	info := updatedKernelInfo["vmlinuz-6.6.92.2-2.azl3"]
	assert.Equal(t, "root=LABEL=ROOT usrhash=newhash", info.Cmdline)
	assert.Nil(t, info.ExistingAddons)
}
