// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAclGetUkiAddonSpecs(t *testing.T) {
	tests := []struct {
		name              string
		cmdline           string
		baseAddons        map[string]string
		expectedSpecs     []UkiAddonSpec
		expectedErr       error
		expectedErrSubstr string
	}{
		{
			name:    "first-boot arg in the middle",
			cmdline: "console=tty0 flatcar.first_boot=detected root=/dev/sda",
			expectedSpecs: []UkiAddonSpec{
				{FileName: "vmlinuz-6.6.92.2-2.azl3.addon.efi", Cmdline: "console=tty0 root=/dev/sda"},
				{FileName: "firstboot.addon.efi", Cmdline: "flatcar.first_boot=detected"},
			},
		},
		{
			name:    "duplicated first-boot args (first, last, adjacent)",
			cmdline: "flatcar.first_boot=detected console=tty0 flatcar.first_boot=detected flatcar.first_boot=detected",
			expectedSpecs: []UkiAddonSpec{
				{FileName: "vmlinuz-6.6.92.2-2.azl3.addon.efi", Cmdline: "console=tty0"},
				{FileName: "firstboot.addon.efi", Cmdline: "flatcar.first_boot=detected"},
			},
		},
		{
			name:    "no first-boot arg (already-booted image)",
			cmdline: "console=tty0 root=/dev/sda",
			expectedSpecs: []UkiAddonSpec{
				{FileName: "vmlinuz-6.6.92.2-2.azl3.addon.efi", Cmdline: "console=tty0 root=/dev/sda"},
			},
		},
		{
			name:    "extra whitespace collapsed",
			cmdline: "console=ttyS0,115200n8   flatcar.first_boot=detected  flatcar.oem.id=azure",
			expectedSpecs: []UkiAddonSpec{
				{FileName: "vmlinuz-6.6.92.2-2.azl3.addon.efi", Cmdline: "console=ttyS0,115200n8 flatcar.oem.id=azure"},
				{FileName: "firstboot.addon.efi", Cmdline: "flatcar.first_boot=detected"},
			},
		},
		{
			name: "similar args preserved",
			cmdline: "myflatcar.first_boot=detected flatcar.first_boot=1 flatcar.first_boot=detected2 " +
				"flatcar.first_boot=detected",
			expectedSpecs: []UkiAddonSpec{
				{
					FileName: "vmlinuz-6.6.92.2-2.azl3.addon.efi",
					Cmdline:  "myflatcar.first_boot=detected flatcar.first_boot=1 flatcar.first_boot=detected2",
				},
				{FileName: "firstboot.addon.efi", Cmdline: "flatcar.first_boot=detected"},
			},
		},
		{
			name:    "oem addon kept",
			cmdline: "root=/dev/sda flatcar.first_boot=detected flatcar.oem.id=azure console=tty1 console=ttyS0,115200n8",
			baseAddons: map[string]string{
				"firstboot.addon.efi": "flatcar.first_boot=detected",
				"oem.addon.efi":       "flatcar.oem.id=azure console=tty1 console=ttyS0,115200n8",
			},
			expectedSpecs: []UkiAddonSpec{
				{FileName: "vmlinuz-6.6.92.2-2.azl3.addon.efi", Cmdline: "root=/dev/sda"},
				{FileName: "oem.addon.efi", Cmdline: "flatcar.oem.id=azure console=tty1 console=ttyS0,115200n8"},
				{FileName: "firstboot.addon.efi", Cmdline: "flatcar.first_boot=detected"},
			},
		},
		{
			name:       "oem addon kept on already-booted image",
			cmdline:    "root=/dev/sda flatcar.oem.id=azure console=tty1",
			baseAddons: map[string]string{"oem.addon.efi": "flatcar.oem.id=azure console=tty1"},
			expectedSpecs: []UkiAddonSpec{
				{FileName: "vmlinuz-6.6.92.2-2.azl3.addon.efi", Cmdline: "root=/dev/sda"},
				{FileName: "oem.addon.efi", Cmdline: "flatcar.oem.id=azure console=tty1"},
			},
		},
		{
			name:       "user copy of an oem arg stays in the persistent addon",
			cmdline:    "root=/dev/sda flatcar.oem.id=azure console=tty1 console=tty1",
			baseAddons: map[string]string{"oem.addon.efi": "flatcar.oem.id=azure console=tty1"},
			expectedSpecs: []UkiAddonSpec{
				{FileName: "vmlinuz-6.6.92.2-2.azl3.addon.efi", Cmdline: "root=/dev/sda console=tty1"},
				{FileName: "oem.addon.efi", Cmdline: "flatcar.oem.id=azure console=tty1"},
			},
		},
		{
			name:       "other base addons are folded into the persistent addon",
			cmdline:    "root=/dev/sda systemd.log_level=debug",
			baseAddons: map[string]string{"debug.addon.efi": "systemd.log_level=debug"},
			expectedSpecs: []UkiAddonSpec{
				{FileName: "vmlinuz-6.6.92.2-2.azl3.addon.efi", Cmdline: "root=/dev/sda systemd.log_level=debug"},
			},
		},
		{
			name:              "empty oem addon",
			cmdline:           "root=/dev/sda",
			baseAddons:        map[string]string{"oem.addon.efi": ""},
			expectedErr:       ErrAclUkiAddonSplit,
			expectedErrSubstr: "empty command line",
		},
		{
			name:        "first-boot arg only",
			cmdline:     "flatcar.first_boot=detected",
			expectedErr: ErrAclUkiAddonEmptyPersistentCmdline,
		},
		{
			name:              "variable expansion",
			cmdline:           "console=tty0 foo=$bar flatcar.first_boot=detected",
			expectedErr:       ErrAclUkiAddonSplit,
			expectedErrSubstr: "variable expansion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specs, err := aclGetUkiAddonSpecs("vmlinuz-6.6.92.2-2.azl3", tt.cmdline, tt.baseAddons)
			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
				if tt.expectedErrSubstr != "" {
					assert.ErrorContains(t, err, tt.expectedErrSubstr)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedSpecs, specs)
		})
	}
}

// TestAclGetUkiAddonSpecsRoundTrip verifies that re-customization converges: the addons merge back in file-name
// order, and splitting that cmdline again yields identical specs.
func TestAclGetUkiAddonSpecsRoundTrip(t *testing.T) {
	kernel := "vmlinuz-6.6.92.2-2.azl3"

	// A stock image: base args in the main UKI, the first-boot and oem addons beside it.
	baseAddons := map[string]string{
		"firstboot.addon.efi": "flatcar.first_boot=detected",
		"oem.addon.efi":       "flatcar.oem.id=azure console=tty1",
	}
	cmdline, err := mergeUkiCmdlineParts("root=/dev/sda", baseAddons)
	assert.NoError(t, err)

	specs, err := aclGetUkiAddonSpecs(kernel, cmdline, baseAddons)
	assert.NoError(t, err)
	assert.Equal(t, []UkiAddonSpec{
		{FileName: "vmlinuz-6.6.92.2-2.azl3.addon.efi", Cmdline: "root=/dev/sda"},
		{FileName: "oem.addon.efi", Cmdline: "flatcar.oem.id=azure console=tty1"},
		{FileName: "firstboot.addon.efi", Cmdline: "flatcar.first_boot=detected"},
	}, specs)

	// The rebuilt image: the main UKI has no cmdline and the addons merge back in file-name order.
	rebuiltAddons := map[string]string{}
	for _, spec := range specs {
		rebuiltAddons[spec.FileName] = spec.Cmdline
	}
	cmdline, err = mergeUkiCmdlineParts("", rebuiltAddons)
	assert.NoError(t, err)
	assert.Equal(t, "flatcar.first_boot=detected flatcar.oem.id=azure console=tty1 root=/dev/sda", cmdline)

	respecs, err := aclGetUkiAddonSpecs(kernel, cmdline, rebuiltAddons)
	assert.NoError(t, err)
	assert.Equal(t, specs, respecs)

	// A booted image has consumed its first-boot addon and keeps the others.
	delete(rebuiltAddons, "firstboot.addon.efi")
	cmdline, err = mergeUkiCmdlineParts("", rebuiltAddons)
	assert.NoError(t, err)

	bootedSpecs, err := aclGetUkiAddonSpecs(kernel, cmdline, rebuiltAddons)
	assert.NoError(t, err)
	assert.Equal(t, specs[:2], bootedSpecs)
}
