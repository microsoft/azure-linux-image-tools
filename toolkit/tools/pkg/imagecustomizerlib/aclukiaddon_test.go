// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// The main UKI command line of a stock ACL image without A/B (verity args in the main UKI).
	testAclMainCmdline = "mount.usr=/dev/mapper/usr mount.usrflags=ro systemd.verity_usr_data=PARTUUID=a1 " +
		"systemd.verity_usr_hash=PARTUUID=a1 systemd.verity_usr_options=hash-offset=1065345024,panic-on-corruption " +
		"usrhash=oldhash root=LABEL=ROOT rootflags=rw consoleblank=0"

	// The main UKI command line of a stock ACL image with A/B (verity args in the slot addon).
	testAclAbMainCmdline = "mount.usr=/dev/mapper/usr mount.usrflags=ro root=LABEL=ROOT rootflags=rw consoleblank=0"
	testAclSlotACmdline  = "systemd.verity_usr_data=PARTUUID=a1 systemd.verity_usr_hash=PARTUUID=a2 " +
		"systemd.verity_usr_options=panic-on-corruption usrhash=oldhash acl.slot=a"
	testAclOemCmdline = "flatcar.oem.id=azure console=tty1 console=ttyS0,115200n8"
)

func TestAclGetUsrHash(t *testing.T) {
	tests := []struct {
		name         string
		kernelInfo   map[string]UkiKernelInfo
		expectedHash string
		expectedErr  string
	}{
		{
			name: "UKIs agree",
			kernelInfo: map[string]UkiKernelInfo{
				"vmlinuz-1": {Cmdline: testAclAbMainCmdline + " usrhash=newhash"},
				"vmlinuz-2": {Cmdline: testAclAbMainCmdline + " usrhash=newhash"},
			},
			expectedHash: "newhash",
		},
		{
			name: "UKIs disagree",
			kernelInfo: map[string]UkiKernelInfo{
				"vmlinuz-1": {Cmdline: testAclAbMainCmdline + " usrhash=newhash"},
				"vmlinuz-2": {Cmdline: testAclAbMainCmdline + " usrhash=otherhash"},
			},
			expectedErr: "UKIs have different /usr root hashes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := aclGetUsrHash(tt.kernelInfo)
			if tt.expectedErr != "" {
				assert.ErrorContains(t, err, tt.expectedErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedHash, hash)
		})
	}
}

func TestAclPlanSlotAddonTemplate(t *testing.T) {
	newSlotACmdline := "systemd.verity_usr_data=PARTUUID=a1 systemd.verity_usr_hash=PARTUUID=a2 " +
		"systemd.verity_usr_options=panic-on-corruption usrhash=newhash acl.slot=a"
	slotBCmdline := "systemd.verity_usr_data=PARTUUID=b1 systemd.verity_usr_hash=PARTUUID=b2 " +
		"systemd.verity_usr_options=panic-on-corruption usrhash=oldhash acl.slot=b"
	newSlotBCmdline := "systemd.verity_usr_data=PARTUUID=b1 systemd.verity_usr_hash=PARTUUID=b2 " +
		"systemd.verity_usr_options=panic-on-corruption usrhash=newhash acl.slot=b"

	tests := []struct {
		name                string
		templateCmdline     string
		activeAddonCmdlines []string
		usrHash             string
		expectedCmdline     string
		expectedCopy        bool
		expectedRebuild     bool
		expectedErr         string
	}{
		{
			name:                "active slot addon is copied over its template",
			templateCmdline:     testAclSlotACmdline,
			activeAddonCmdlines: []string{newSlotACmdline},
			usrHash:             "newhash",
			expectedCmdline:     newSlotACmdline,
			expectedCopy:        true,
		},
		{
			name:                "active slot addon is copied when the root hash did not change",
			templateCmdline:     newSlotACmdline,
			activeAddonCmdlines: []string{newSlotACmdline},
			usrHash:             "newhash",
			expectedCmdline:     newSlotACmdline,
			expectedCopy:        true,
		},
		{
			name:                "identical addons of several UKIs",
			templateCmdline:     testAclSlotACmdline,
			activeAddonCmdlines: []string{newSlotACmdline, newSlotACmdline},
			usrHash:             "newhash",
			expectedCmdline:     newSlotACmdline,
			expectedCopy:        true,
		},
		{
			name:            "inactive slot template is rebuilt",
			templateCmdline: slotBCmdline,
			usrHash:         "newhash",
			expectedCmdline: newSlotBCmdline,
			expectedRebuild: true,
		},
		{
			name:            "template already up to date",
			templateCmdline: newSlotBCmdline,
			usrHash:         "newhash",
			expectedCmdline: newSlotBCmdline,
		},
		{
			name:                "active addon differs from its template beyond the root hash",
			templateCmdline:     testAclSlotACmdline,
			activeAddonCmdlines: []string{newSlotACmdline, newSlotACmdline + " extra=1"},
			usrHash:             "newhash",
			expectedErr:         "A/B slot addon args can't change apart from the root hash",
		},
		{
			name:            "rebuilt UKIs without a root hash",
			templateCmdline: slotBCmdline,
			expectedErr:     "the rebuilt UKIs have no (usrhash) arg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdline, copyActiveAddon, rebuild, err := aclPlanSlotAddonTemplate(tt.templateCmdline,
				tt.activeAddonCmdlines, tt.usrHash)
			if tt.expectedErr != "" {
				assert.ErrorContains(t, err, tt.expectedErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCmdline, cmdline)
			assert.Equal(t, tt.expectedCopy, copyActiveAddon)
			assert.Equal(t, tt.expectedRebuild, rebuild)
		})
	}
}

func TestAclValidateUkiModify(t *testing.T) {
	abLayout := UkiLayout{
		MainCmdline: testAclAbMainCmdline,
		Addons:      map[string]string{"oem.addon.efi": testAclOemCmdline, "slot-a.addon.efi": testAclSlotACmdline},
	}
	layout := UkiLayout{MainCmdline: testAclMainCmdline, Addons: map[string]string{"oem.addon.efi": testAclOemCmdline}}
	verityCmdline := "rd.systemd.verity=1 usrhash=newhash"

	assert.ErrorIs(t, aclValidateUkiModify(map[string]UkiKernelInfo{
		"vmlinuz-1": {Cmdline: verityCmdline, BaseLayout: &abLayout},
	}), ErrAclUkiModifyRootHash)
	assert.NoError(t, aclValidateUkiModify(map[string]UkiKernelInfo{
		"vmlinuz-1": {Cmdline: "console=ttyS1", BaseLayout: &abLayout},
		"vmlinuz-2": {Cmdline: verityCmdline, BaseLayout: &layout},
	}))
}

// TestAclRebuildUkiOnEsp builds a stock ACL ESP with ukify and runs the create-mode steps that keep its layout: record
// the layout, check it for changes, rebuild the UKI with a new root hash and update the slot addon templates.
func TestAclRebuildUkiOnEsp(t *testing.T) {
	stubPath, addonStubPath := checkSkipForUkiStubs(t)

	kernel := "vmlinuz-6.6.157.1-1.azl3"
	buildDir := t.TempDir()
	espDir := t.TempDir()
	ukiFile := filepath.Join(espDir, UkiOutputDir, kernel+".efi")
	addonDir := ukiFile + ".extra.d"
	templateDir := filepath.Join(espDir, aclUkiAddonTemplatesDir)
	osReleasePath := filepath.Join(buildDir, "os-release")

	cred := []byte("signed policy")
	credHash := fmt.Sprintf("%x", sha256.Sum256(cred))
	mainCmdline := testAclAbMainCmdline + " acl.ipe.policy_sha256=" + credHash
	slotBCmdline := "systemd.verity_usr_data=PARTUUID=b1 systemd.verity_usr_hash=PARTUUID=b2 " +
		"systemd.verity_usr_options=panic-on-corruption usrhash=oldhash acl.slot=b"
	baseAddons := map[string]string{
		"firstboot.addon.efi": "flatcar.first_boot=detected",
		"oem.addon.efi":       testAclOemCmdline,
		"slot-a.addon.efi":    testAclSlotACmdline,
	}

	// The base image's ESP, built the way ACL builds it: from command-line files written with echo, so each command
	// line ends with a newline. Any PE file can stand in for the kernel, since ukify only embeds it.
	require.NoError(t, os.MkdirAll(filepath.Join(buildDir, UkiBuildDir), os.ModePerm))
	require.NoError(t, os.MkdirAll(addonDir, os.ModePerm))
	require.NoError(t, os.MkdirAll(templateDir, os.ModePerm))
	require.NoError(t, file.Copy(stubPath, filepath.Join(buildDir, UkiBuildDir, kernel)))
	require.NoError(t, os.WriteFile(filepath.Join(buildDir, UkiBuildDir, "initramfs.img"), []byte("initramfs"), 0o644))
	require.NoError(t, os.WriteFile(osReleasePath, []byte("ID=azurelinux\n"), 0o644))
	require.NoError(t, buildMainUki(kernel, "initramfs.img", mainCmdline+"\n", osReleasePath, stubPath, buildDir,
		espDir, "6.6.157.1-1.azl3"))

	cmdlineFile := filepath.Join(buildDir, "cmdline.txt")
	buildAddon := func(path string, cmdline string) {
		require.NoError(t, os.WriteFile(cmdlineFile, []byte(cmdline+"\n"), 0o644))
		require.NoError(t, buildUkiAddonFile(path, "@"+cmdlineFile, addonStubPath))
	}
	for name, cmdline := range baseAddons {
		buildAddon(filepath.Join(addonDir, name), cmdline)
	}
	buildAddon(filepath.Join(templateDir, "firstboot.addon.efi"), "flatcar.first_boot=detected")
	buildAddon(filepath.Join(templateDir, "slot-b.addon.efi"), slotBCmdline)
	require.NoError(t, file.Copy(filepath.Join(addonDir, "slot-a.addon.efi"),
		filepath.Join(templateDir, "slot-a.addon.efi")))
	require.NoError(t, os.WriteFile(filepath.Join(addonDir, "acl-ipe-policy.p7b.cred"), cred, 0o644))

	// Record the layout, and check it for changes.
	kernelInfo, err := readUkiBaseLayouts(espDir, buildDir)
	require.NoError(t, err)
	assert.Equal(t, &UkiLayout{
		MainCmdline: mainCmdline,
		Addons:      baseAddons,
		ExtraFiles:  map[string]string{"acl-ipe-policy.p7b.cred": credHash},
	}, kernelInfo[kernel].BaseLayout)
	assert.NoError(t, checkUkiLayoutsUnchanged(espDir, kernelInfo, buildDir))

	kdumpAddonPath := filepath.Join(addonDir, "kdump.addon.efi")
	require.NoError(t, buildUkiAddonFile(kdumpAddonPath, "crashkernel=256M", addonStubPath))
	err = checkUkiLayoutsUnchanged(espDir, kernelInfo, buildDir)
	assert.ErrorIs(t, err, ErrUKIAddonDirChanged)
	assert.ErrorContains(t, err, "changes='added kdump.addon.efi'")
	require.NoError(t, os.Remove(kdumpAddonPath))

	// Rebuild the UKI with a new root hash, the way storage.reinitializeVerity does.
	kernelInfoPath := filepath.Join(buildDir, UkiBuildDir, UkiKernelInfoJson)
	require.NoError(t, writeUkiKernelInfoFile(kernelInfoPath, kernelInfo))
	require.NoError(t, appendKernelArgsToUkiCmdlineFile(buildDir, []string{"rd.systemd.verity=1", "usrhash=newhash",
		"systemd.verity_usr_data=PARTUUID=a1", "systemd.verity_usr_hash=PARTUUID=a2",
		"systemd.verity_usr_options=panic-on-corruption"}))
	kernelInfo, err = readUkiKernelInfoFile(kernelInfoPath)
	require.NoError(t, err)

	info := kernelInfo[kernel]
	info.Initramfs = "initramfs.img"
	distroHandler := newAclDistroHandler(targetos.TargetOsAzureContainerLinux3)
	require.NoError(t, cleanUkiDirectory(filepath.Dir(ukiFile)))
	require.NoError(t, buildUki(kernel, info, osReleasePath, stubPath, addonStubPath, buildDir, espDir))
	require.NoError(t, distroHandler.FinalizeUkis(espDir, addonStubPath, kernelInfo, buildDir))

	// The token stays in the main UKI, the new root hash takes the old one's place in the slot addon and its
	// templates, the credential is put back, and the first-boot template (built with a newline) is left alone.
	newSlotACmdline := strings.ReplaceAll(testAclSlotACmdline, "usrhash=oldhash", "usrhash=newhash")
	newSlotBCmdline := strings.ReplaceAll(slotBCmdline, "usrhash=oldhash", "usrhash=newhash")
	expectedCmdlines := map[string]string{
		ukiFile: mainCmdline + " rd.systemd.verity=1",
		filepath.Join(addonDir, "firstboot.addon.efi"):    "flatcar.first_boot=detected",
		filepath.Join(addonDir, "oem.addon.efi"):          testAclOemCmdline,
		filepath.Join(addonDir, "slot-a.addon.efi"):       newSlotACmdline,
		filepath.Join(templateDir, "firstboot.addon.efi"): "flatcar.first_boot=detected\n",
		filepath.Join(templateDir, "slot-b.addon.efi"):    newSlotBCmdline,
	}
	for path, expectedCmdline := range expectedCmdlines {
		cmdline, err := extractCmdlineFromSinglePE(path, buildDir)
		assert.NoError(t, err)
		assert.Equal(t, expectedCmdline, cmdline, path)
	}

	entries, err := os.ReadDir(addonDir)
	require.NoError(t, err)
	names := []string(nil)
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	assert.Equal(t, []string{"acl-ipe-policy.p7b.cred", "firstboot.addon.efi", "oem.addon.efi", "slot-a.addon.efi"},
		names)

	content, err := os.ReadFile(filepath.Join(addonDir, "acl-ipe-policy.p7b.cred"))
	assert.NoError(t, err)
	assert.Equal(t, cred, content)

	activeSlotAddon, err := os.ReadFile(filepath.Join(addonDir, "slot-a.addon.efi"))
	assert.NoError(t, err)
	slotATemplate, err := os.ReadFile(filepath.Join(templateDir, "slot-a.addon.efi"))
	assert.NoError(t, err)
	assert.Equal(t, activeSlotAddon, slotATemplate)
}
