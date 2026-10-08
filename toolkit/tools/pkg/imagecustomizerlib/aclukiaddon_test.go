// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestAclGetUkiLayout(t *testing.T) {
	tests := []struct {
		name           string
		cmdline        string
		baseLayout     *UkiLayout
		expectedLayout UkiLayout
		expectedErr    error
	}{
		{
			name: "verity refresh keeps the args of a stock image in the main UKI",
			// The command line after Image Customizer replaced the verity args.
			cmdline: "mount.usr=/dev/mapper/usr mount.usrflags=ro root=LABEL=ROOT rootflags=rw consoleblank=0 " +
				"flatcar.first_boot=detected " + testAclOemCmdline + " rd.systemd.verity=1 usrhash=newhash " +
				"systemd.verity_usr_data=PARTUUID=a1 systemd.verity_usr_hash=PARTUUID=a1 " +
				"systemd.verity_usr_options=panic-on-corruption,hash-offset=1065345024",
			baseLayout: &UkiLayout{
				MainCmdline: testAclMainCmdline,
				Addons: map[string]string{
					"firstboot.addon.efi": "flatcar.first_boot=detected",
					"oem.addon.efi":       testAclOemCmdline,
				},
			},
			expectedLayout: UkiLayout{
				MainCmdline: "mount.usr=/dev/mapper/usr mount.usrflags=ro systemd.verity_usr_data=PARTUUID=a1 " +
					"systemd.verity_usr_hash=PARTUUID=a1 " +
					"systemd.verity_usr_options=panic-on-corruption,hash-offset=1065345024 usrhash=newhash " +
					"root=LABEL=ROOT rootflags=rw consoleblank=0 rd.systemd.verity=1",
				Addons: map[string]string{
					"firstboot.addon.efi": "flatcar.first_boot=detected",
					"oem.addon.efi":       testAclOemCmdline,
				},
			},
		},
		{
			name: "verity refresh updates the A/B slot addon",
			cmdline: testAclAbMainCmdline + " flatcar.first_boot=detected " + testAclOemCmdline + " acl.slot=a " +
				"rd.systemd.verity=1 usrhash=newhash systemd.verity_usr_data=PARTUUID=a1 " +
				"systemd.verity_usr_hash=PARTUUID=a2 systemd.verity_usr_options=panic-on-corruption",
			baseLayout: &UkiLayout{
				MainCmdline: testAclAbMainCmdline,
				Addons: map[string]string{
					"firstboot.addon.efi": "flatcar.first_boot=detected",
					"oem.addon.efi":       testAclOemCmdline,
					"slot-a.addon.efi":    testAclSlotACmdline,
				},
			},
			expectedLayout: UkiLayout{
				MainCmdline: testAclAbMainCmdline + " rd.systemd.verity=1",
				Addons: map[string]string{
					"firstboot.addon.efi": "flatcar.first_boot=detected",
					"oem.addon.efi":       testAclOemCmdline,
					"slot-a.addon.efi": "systemd.verity_usr_data=PARTUUID=a1 systemd.verity_usr_hash=PARTUUID=a2 " +
						"systemd.verity_usr_options=panic-on-corruption usrhash=newhash acl.slot=a",
				},
			},
		},
		{
			name: "IPE policy token and credential stay with the main UKI",
			cmdline: testAclAbMainCmdline + " acl.ipe.policy_sha256=abc flatcar.first_boot=detected " +
				testAclOemCmdline,
			baseLayout: &UkiLayout{
				MainCmdline: testAclAbMainCmdline + " acl.ipe.policy_sha256=abc",
				Addons: map[string]string{
					"firstboot.addon.efi": "flatcar.first_boot=detected",
					"oem.addon.efi":       testAclOemCmdline,
				},
				ExtraFiles: map[string]string{"acl-ipe-policy.p7b.cred": "credhash"},
			},
			expectedLayout: UkiLayout{
				MainCmdline: testAclAbMainCmdline + " acl.ipe.policy_sha256=abc",
				Addons: map[string]string{
					"firstboot.addon.efi": "flatcar.first_boot=detected",
					"oem.addon.efi":       testAclOemCmdline,
				},
				ExtraFiles: map[string]string{"acl-ipe-policy.p7b.cred": "credhash"},
			},
		},
		{
			name:    "new args go to the main UKI, or after the args of the same name they override",
			cmdline: testAclAbMainCmdline + " " + testAclOemCmdline + " console=ttyS1 security=selinux selinux=1 rw",
			baseLayout: &UkiLayout{
				MainCmdline: testAclAbMainCmdline,
				Addons:      map[string]string{"oem.addon.efi": testAclOemCmdline},
			},
			expectedLayout: UkiLayout{
				MainCmdline: testAclAbMainCmdline + " security=selinux selinux=1 rw",
				Addons:      map[string]string{"oem.addon.efi": testAclOemCmdline + " console=ttyS1"},
			},
		},
		{
			name:    "only the first-boot arg",
			cmdline: "flatcar.first_boot=detected",
			baseLayout: &UkiLayout{
				Addons: map[string]string{"firstboot.addon.efi": "flatcar.first_boot=detected"},
			},
			expectedErr: ErrAclUkiAddonEmptyPersistentCmdline,
		},
		{
			name:    "base addon without a command line",
			cmdline: testAclAbMainCmdline,
			baseLayout: &UkiLayout{
				MainCmdline: testAclAbMainCmdline,
				Addons:      map[string]string{"devicetree.addon.efi": ""},
			},
			expectedErr: ErrAclUkiAddonSplit,
		},
		{
			name:    "changed values stay in their addon, in order",
			cmdline: testAclAbMainCmdline + " flatcar.oem.id=azure console=tty2 console=ttyS1",
			baseLayout: &UkiLayout{
				MainCmdline: testAclAbMainCmdline,
				Addons:      map[string]string{"oem.addon.efi": testAclOemCmdline},
			},
			expectedLayout: UkiLayout{
				MainCmdline: testAclAbMainCmdline,
				Addons:      map[string]string{"oem.addon.efi": "flatcar.oem.id=azure console=tty2 console=ttyS1"},
			},
		},
		{
			name:    "removed args are dropped and an empty addon is not added",
			cmdline: testAclAbMainCmdline + " flatcar.oem.id=qemu",
			baseLayout: &UkiLayout{
				MainCmdline: testAclAbMainCmdline,
				Addons: map[string]string{
					"debug.addon.efi": "systemd.log_level=debug",
					"oem.addon.efi":   "flatcar.oem.id=qemu flatcar.autologin",
				},
			},
			expectedLayout: UkiLayout{
				MainCmdline: testAclAbMainCmdline,
				Addons:      map[string]string{"oem.addon.efi": "flatcar.oem.id=qemu"},
			},
		},
		{
			name:    "booted image has no first-boot addon",
			cmdline: testAclAbMainCmdline + " " + testAclOemCmdline,
			baseLayout: &UkiLayout{
				MainCmdline: testAclAbMainCmdline,
				Addons: map[string]string{
					"firstboot.addon.efi": "flatcar.first_boot=detected",
					"oem.addon.efi":       testAclOemCmdline,
				},
			},
			expectedLayout: UkiLayout{
				MainCmdline: testAclAbMainCmdline,
				Addons:      map[string]string{"oem.addon.efi": testAclOemCmdline},
			},
		},
		{
			name: "first-boot arg always goes to the first-boot addon",
			cmdline: "flatcar.first_boot=detected " + testAclAbMainCmdline + " flatcar.first_boot=detected " +
				"flatcar.first_boot=1 myflatcar.first_boot=detected",
			baseLayout: &UkiLayout{MainCmdline: testAclAbMainCmdline},
			expectedLayout: UkiLayout{
				MainCmdline: testAclAbMainCmdline + " flatcar.first_boot=1 myflatcar.first_boot=detected",
				Addons:      map[string]string{"firstboot.addon.efi": "flatcar.first_boot=detected"},
			},
		},
		{
			name:    "args of an image built by an older Image Customizer stay in its addon",
			cmdline: "flatcar.first_boot=detected root=/dev/sda console=tty0 rd.info",
			baseLayout: &UkiLayout{
				Addons: map[string]string{
					"firstboot.addon.efi":               "flatcar.first_boot=detected",
					"vmlinuz-6.6.92.2-2.azl3.addon.efi": "root=/dev/sda console=tty0",
				},
			},
			expectedLayout: UkiLayout{
				MainCmdline: "rd.info",
				Addons: map[string]string{
					"firstboot.addon.efi":               "flatcar.first_boot=detected",
					"vmlinuz-6.6.92.2-2.azl3.addon.efi": "root=/dev/sda console=tty0",
				},
			},
		},
		{
			name:    "without a base layout the command line goes to the main UKI",
			cmdline: "root=/dev/sda flatcar.first_boot=detected console=tty0",
			expectedLayout: UkiLayout{
				MainCmdline: "root=/dev/sda console=tty0",
				Addons:      map[string]string{"firstboot.addon.efi": "flatcar.first_boot=detected"},
			},
		},
		{
			name:    "changed arg found in two files",
			cmdline: "root=/dev/sda x=3",
			baseLayout: &UkiLayout{
				MainCmdline: "root=/dev/sda x=1",
				Addons:      map[string]string{"y.addon.efi": "x=2"},
			},
			expectedErr: ErrAclUkiAddonAmbiguousArg,
		},
		{
			name:        "variable expansion",
			cmdline:     "root=/dev/sda console=$console",
			expectedErr: ErrAclUkiAddonSplit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout, err := aclGetUkiLayout(tt.cmdline, tt.baseLayout)
			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedLayout, layout)
		})
	}
}

// TestAclGetUkiLayoutRoundTrip verifies that re-customization converges: splitting the command line that a rebuilt
// UKI boots with, using that UKI as the base, gives the same layout.
func TestAclGetUkiLayoutRoundTrip(t *testing.T) {
	baseLayout := UkiLayout{
		MainCmdline: testAclAbMainCmdline + " acl.ipe.policy_sha256=abc",
		Addons: map[string]string{
			"firstboot.addon.efi": "flatcar.first_boot=detected",
			"oem.addon.efi":       testAclOemCmdline,
			"slot-a.addon.efi":    testAclSlotACmdline,
		},
	}

	cmdline, err := mergeUkiCmdlineParts(baseLayout.MainCmdline, baseLayout.Addons)
	assert.NoError(t, err)

	layout, err := aclGetUkiLayout(cmdline, &baseLayout)
	assert.NoError(t, err)
	assert.Equal(t, baseLayout, layout)

	cmdline, err = mergeUkiCmdlineParts(layout.MainCmdline, layout.Addons)
	assert.NoError(t, err)

	relayout, err := aclGetUkiLayout(cmdline, &layout)
	assert.NoError(t, err)
	assert.Equal(t, layout, relayout)
}

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
			name: "no root hash",
			kernelInfo: map[string]UkiKernelInfo{
				"vmlinuz-1": {Cmdline: testAclAbMainCmdline},
			},
		},
		{
			name: "UKIs disagree",
			kernelInfo: map[string]UkiKernelInfo{
				"vmlinuz-1": {Cmdline: testAclAbMainCmdline + " usrhash=newhash"},
				"vmlinuz-2": {Cmdline: testAclAbMainCmdline + " usrhash=otherhash"},
			},
			expectedErr: "UKIs have different /usr root hashes",
		},
		{
			name: "duplicate root hash",
			kernelInfo: map[string]UkiKernelInfo{
				"vmlinuz-1": {Cmdline: "usrhash=a usrhash=b"},
			},
			expectedErr: "more than one (usrhash) arg",
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
			name:            "stale template of an older image gets the new root hash",
			templateCmdline: strings.ReplaceAll(slotBCmdline, "oldhash", "stalehash"),
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
			activeAddonCmdlines: []string{newSlotACmdline + " extra=1"},
			usrHash:             "newhash",
			expectedErr:         "rebuilt addon does not match its template",
		},
		{
			name:                "addons of several UKIs differ",
			templateCmdline:     testAclSlotACmdline,
			activeAddonCmdlines: []string{newSlotACmdline, strings.ReplaceAll(newSlotACmdline, "a2", "a3")},
			usrHash:             "newhash",
			expectedErr:         "rebuilt addon does not match its template",
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

func TestAclSetArgValue(t *testing.T) {
	cmdline, changed, err := aclSetArgValue(testAclSlotACmdline, "usrhash", "newhash")
	assert.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, "systemd.verity_usr_data=PARTUUID=a1 systemd.verity_usr_hash=PARTUUID=a2 "+
		"systemd.verity_usr_options=panic-on-corruption usrhash=newhash acl.slot=a", cmdline)

	cmdline, changed, err = aclSetArgValue(cmdline, "usrhash", "newhash")
	assert.NoError(t, err)
	assert.False(t, changed)

	cmdline, changed, err = aclSetArgValue("fips=1", "usrhash", "newhash")
	assert.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, "fips=1", cmdline)
}
