// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetUkiLayout(t *testing.T) {
	tests := []struct {
		name           string
		cmdline        string
		baseLayout     *UkiLayout
		expectedLayout UkiLayout
		expectedErr    error
	}{
		{
			name: "verity refresh keeps the args of a stock ACL image in the main UKI",
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
			name: "verity refresh updates the A/B slot addon, and the IPE token and credential stay",
			cmdline: testAclAbMainCmdline + " acl.ipe.policy_sha256=abc flatcar.first_boot=detected " +
				testAclOemCmdline + " acl.slot=a rd.systemd.verity=1 usrhash=newhash " +
				"systemd.verity_usr_data=PARTUUID=a1 systemd.verity_usr_hash=PARTUUID=a2 " +
				"systemd.verity_usr_options=panic-on-corruption",
			baseLayout: &UkiLayout{
				MainCmdline: testAclAbMainCmdline + " acl.ipe.policy_sha256=abc",
				Addons: map[string]string{
					"firstboot.addon.efi": "flatcar.first_boot=detected",
					"oem.addon.efi":       testAclOemCmdline,
					"slot-a.addon.efi":    testAclSlotACmdline,
				},
				ExtraFiles: map[string]string{"acl-ipe-policy.p7b.cred": "credhash"},
			},
			expectedLayout: UkiLayout{
				MainCmdline: testAclAbMainCmdline + " acl.ipe.policy_sha256=abc rd.systemd.verity=1",
				Addons: map[string]string{
					"firstboot.addon.efi": "flatcar.first_boot=detected",
					"oem.addon.efi":       testAclOemCmdline,
					"slot-a.addon.efi": "systemd.verity_usr_data=PARTUUID=a1 systemd.verity_usr_hash=PARTUUID=a2 " +
						"systemd.verity_usr_options=panic-on-corruption usrhash=newhash acl.slot=a",
				},
				ExtraFiles: map[string]string{"acl-ipe-policy.p7b.cred": "credhash"},
			},
		},
		{
			name: "new args go to the main UKI, or after the args of the same name they override",
			cmdline: testAclAbMainCmdline + " " + testAclOemCmdline +
				" console=ttyS1 security=selinux selinux=1 rw rootflags=ro",
			baseLayout: &UkiLayout{
				MainCmdline: testAclAbMainCmdline,
				Addons:      map[string]string{"oem.addon.efi": testAclOemCmdline},
			},
			expectedLayout: UkiLayout{
				MainCmdline: testAclAbMainCmdline + " security=selinux selinux=1 rw rootflags=ro",
				Addons:      map[string]string{"oem.addon.efi": testAclOemCmdline + " console=ttyS1"},
			},
		},
		{
			name:    "base addon without a command line",
			cmdline: testAclAbMainCmdline,
			baseLayout: &UkiLayout{
				MainCmdline: testAclAbMainCmdline,
				Addons:      map[string]string{"devicetree.addon.efi": ""},
			},
			expectedErr: ErrUKISplitCmdline,
		},
		{
			name:    "changed values stay in order, and addons left empty are dropped",
			cmdline: testAclAbMainCmdline + " flatcar.oem.id=azure console=tty2 console=ttyS1",
			baseLayout: &UkiLayout{
				MainCmdline: testAclAbMainCmdline,
				Addons: map[string]string{
					"debug.addon.efi":     "systemd.log_level=debug",
					"firstboot.addon.efi": "flatcar.first_boot=detected",
					"oem.addon.efi":       testAclOemCmdline,
				},
			},
			expectedLayout: UkiLayout{
				MainCmdline: testAclAbMainCmdline,
				Addons:      map[string]string{"oem.addon.efi": "flatcar.oem.id=azure console=tty2 console=ttyS1"},
			},
		},
		{
			name:    "args of an image built by an older Image Customizer stay in its addon",
			cmdline: "root=/dev/sda console=tty0 rd.info",
			baseLayout: &UkiLayout{
				Addons: map[string]string{"vmlinuz-6.6.92.2-2.azl3.addon.efi": "root=/dev/sda console=tty0"},
			},
			expectedLayout: UkiLayout{
				MainCmdline: "rd.info",
				Addons:      map[string]string{"vmlinuz-6.6.92.2-2.azl3.addon.efi": "root=/dev/sda console=tty0"},
			},
		},
		{
			name:           "without a base layout the command line goes to the main UKI",
			cmdline:        "root=/dev/sda console=tty0",
			expectedLayout: UkiLayout{MainCmdline: "root=/dev/sda console=tty0", Addons: map[string]string{}},
		},
		{
			name:    "changed arg found in two files",
			cmdline: "root=/dev/sda x=3",
			baseLayout: &UkiLayout{
				MainCmdline: "root=/dev/sda x=1",
				Addons:      map[string]string{"y.addon.efi": "x=2"},
			},
			expectedErr: ErrUKIAmbiguousArg,
		},
		{
			name:        "variable expansion",
			cmdline:     "root=/dev/sda console=$console",
			expectedErr: ErrUKISplitCmdline,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout, err := getUkiLayout(tt.cmdline, tt.baseLayout)
			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedLayout, layout)
		})
	}
}

// TestGetUkiLayoutRoundTrip verifies that re-customization converges: splitting the command line that a rebuilt
// UKI boots with, using that UKI as the base, gives the same layout.
func TestGetUkiLayoutRoundTrip(t *testing.T) {
	baseLayout := UkiLayout{
		MainCmdline: testAclAbMainCmdline + " acl.ipe.policy_sha256=abc",
		Addons: map[string]string{
			"firstboot.addon.efi": "flatcar.first_boot=detected",
			"oem.addon.efi":       testAclOemCmdline,
			"slot-a.addon.efi":    testAclSlotACmdline,
		},
	}

	baseCmdline, err := mergeUkiCmdlineParts(baseLayout.MainCmdline, baseLayout.Addons)
	assert.NoError(t, err)

	layout, err := getUkiLayout(baseCmdline, &baseLayout)
	assert.NoError(t, err)
	assert.Equal(t, baseLayout, layout)

	// The base command line as is, and after a verity refresh changed the root hash.
	cmdlines := []string{
		baseCmdline,
		strings.ReplaceAll(baseCmdline, "usrhash=oldhash", "usrhash=newhash") + " rd.systemd.verity=1",
	}
	for _, cmdline := range cmdlines {
		layout, err := getUkiLayout(cmdline, &baseLayout)
		assert.NoError(t, err)

		rebuiltCmdline, err := mergeUkiCmdlineParts(layout.MainCmdline, layout.Addons)
		assert.NoError(t, err)

		relayout, err := getUkiLayout(rebuiltCmdline, &layout)
		assert.NoError(t, err)
		assert.Equal(t, layout, relayout)
	}
}

func TestGetFallbackUkiLayout(t *testing.T) {
	sharedLayout := UkiLayout{
		MainCmdline: "root=/dev/sda",
		Addons:      map[string]string{"oem.addon.efi": "flatcar.oem.id=azure"},
		ExtraFiles:  map[string]string{"policy.cred": "abc"},
	}

	tests := []struct {
		name           string
		existingUkis   map[string]UkiKernelInfo
		expectedLayout *UkiLayout
		expectError    bool
	}{
		{
			name: "shared layout",
			existingUkis: map[string]UkiKernelInfo{
				"vmlinuz-1": {BaseLayout: &sharedLayout},
				"vmlinuz-2": {BaseLayout: &sharedLayout},
			},
			expectedLayout: &sharedLayout,
		},
		{
			name: "each UKI's own Image Customizer addon is renamed for the new kernel",
			existingUkis: map[string]UkiKernelInfo{
				"vmlinuz-1": {BaseLayout: &UkiLayout{Addons: map[string]string{"vmlinuz-1.addon.efi": "rw"}}},
				"vmlinuz-2": {BaseLayout: &UkiLayout{Addons: map[string]string{"vmlinuz-2.addon.efi": "rw"}}},
			},
			expectedLayout: &UkiLayout{Addons: map[string]string{"vmlinuz-3.addon.efi": "rw"}},
		},
		{
			name: "divergent layouts",
			existingUkis: map[string]UkiKernelInfo{
				"vmlinuz-1": {BaseLayout: &sharedLayout},
				"vmlinuz-2": {BaseLayout: &UkiLayout{MainCmdline: "root=/dev/sdb"}},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout, err := getFallbackUkiLayout(tt.existingUkis, "vmlinuz-3")
			if tt.expectError {
				assert.ErrorContains(t, err, "divergent UKI layouts")
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedLayout, layout)
		})
	}
}

func TestUkiLayoutChanges(t *testing.T) {
	baseLayout := &UkiLayout{
		MainCmdline: "rw",
		Addons:      map[string]string{"oem.addon.efi": "console=tty0", "slot-a.addon.efi": "usrhash=a"},
		ExtraFiles:  map[string]string{"policy.cred": "hash1"},
	}

	assert.True(t, ukiLayoutsEqual(nil, nil))
	assert.False(t, ukiLayoutsEqual(baseLayout, nil))
	assert.True(t, ukiLayoutsEqual(&UkiLayout{MainCmdline: "rw"}, &UkiLayout{MainCmdline: "rw",
		Addons: map[string]string{}, ExtraFiles: map[string]string{}}))
	assert.True(t, ukiLayoutsEqual(baseLayout, baseLayout))
	assert.Empty(t, ukiLayoutChanges(baseLayout, baseLayout))
	assert.Equal(t, []string{"main UKI command line", "added kdump.addon.efi", "changed slot-a.addon.efi",
		"removed oem.addon.efi", "changed policy.cred"},
		ukiLayoutChanges(baseLayout, &UkiLayout{
			MainCmdline: "ro",
			Addons:      map[string]string{"kdump.addon.efi": "crashkernel=256M", "slot-a.addon.efi": "usrhash=b"},
			ExtraFiles:  map[string]string{"policy.cred": "hash2"},
		}))
	assert.Equal(t, []string{"removed policy.cred"},
		ukiLayoutChanges(baseLayout, &UkiLayout{MainCmdline: "rw", Addons: baseLayout.Addons}))
}
