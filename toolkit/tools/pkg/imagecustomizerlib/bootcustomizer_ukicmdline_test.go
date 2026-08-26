// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestBootCustomizerAppendToUkiCmdlineFilePreservesExistingAddons verifies that appending a
// kernel command-line argument via appendToUkiCmdlineFile does not discard a kernel's
// ExistingAddons. Losing ExistingAddons here would force every kernelCommandLine customization
// through DistroHandler.GetUkiAddonSpecs' "no existing addons" fallback (a full re-split),
// instead of ever exercising the addon-preserving path (e.g. ACL's aclGetUkiAddonSpecsPreserving
// and its customized.addon.efi handling).
func TestBootCustomizerAppendToUkiCmdlineFilePreservesExistingAddons(t *testing.T) {
	testTempDir := t.TempDir()
	kernelInfoPath := filepath.Join(testTempDir, "uki-kernel-info.json")

	existingAddons := map[string]string{
		ukiMainCmdlineAddonKey: "root=LABEL=ROOT",
		"oem.addon.efi":        "flatcar.oem.id=azure",
	}
	initialKernelInfo := map[string]UkiKernelInfo{
		"vmlinuz-6.6.92.2-2.azl3": {
			Cmdline:        "root=LABEL=ROOT flatcar.oem.id=azure",
			Initramfs:      "initrd-6.6.92.2-2.azl3",
			ExistingAddons: existingAddons,
		},
	}
	err := writeUkiKernelInfoFile(kernelInfoPath, initialKernelInfo)
	assert.NoError(t, err)

	b := &BootCustomizer{ukiKernelInfoPath: kernelInfoPath}
	err = b.appendToUkiCmdlineFile("mitigations=off")
	assert.NoError(t, err)

	updatedKernelInfo, err := readUkiKernelInfoFile(kernelInfoPath)
	assert.NoError(t, err)

	info := updatedKernelInfo["vmlinuz-6.6.92.2-2.azl3"]
	assert.Equal(t, "root=LABEL=ROOT flatcar.oem.id=azure mitigations=off", info.Cmdline)
	assert.Equal(t, "initrd-6.6.92.2-2.azl3", info.Initramfs)
	assert.Equal(t, existingAddons, info.ExistingAddons)
}

// TestBootCustomizerUpdateUkiCmdlineFilePreservesExistingAddons verifies the same for
// updateUkiCmdlineFile (used for arg removal + replacement, e.g. SELinux mode changes).
func TestBootCustomizerUpdateUkiCmdlineFilePreservesExistingAddons(t *testing.T) {
	testTempDir := t.TempDir()
	kernelInfoPath := filepath.Join(testTempDir, "uki-kernel-info.json")

	existingAddons := map[string]string{
		ukiMainCmdlineAddonKey: "root=LABEL=ROOT selinux=0",
	}
	initialKernelInfo := map[string]UkiKernelInfo{
		"vmlinuz-6.6.92.2-2.azl3": {
			Cmdline:        "root=LABEL=ROOT selinux=0",
			Initramfs:      "initrd-6.6.92.2-2.azl3",
			ExistingAddons: existingAddons,
		},
	}
	err := writeUkiKernelInfoFile(kernelInfoPath, initialKernelInfo)
	assert.NoError(t, err)

	b := &BootCustomizer{ukiKernelInfoPath: kernelInfoPath}
	err = b.updateUkiCmdlineFile([]string{"selinux"}, []string{"selinux=1"})
	assert.NoError(t, err)

	updatedKernelInfo, err := readUkiKernelInfoFile(kernelInfoPath)
	assert.NoError(t, err)

	info := updatedKernelInfo["vmlinuz-6.6.92.2-2.azl3"]
	assert.Equal(t, "root=LABEL=ROOT selinux=1", info.Cmdline)
	assert.Equal(t, "initrd-6.6.92.2-2.azl3", info.Initramfs)
	assert.Equal(t, existingAddons, info.ExistingAddons)
}
