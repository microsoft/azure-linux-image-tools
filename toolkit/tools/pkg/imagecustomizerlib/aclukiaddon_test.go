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
			name:        "first-boot arg only",
			cmdline:     "flatcar.first_boot=detected",
			expectedErr: ErrAclUkiAddonEmptyPersistentCmdline,
		},
		{
			name: "verity args split into their own addon",
			cmdline: "mount.usr=/dev/mapper/usr mount.usrflags=ro " +
				"systemd.verity_usr_data=PARTUUID=aaaa systemd.verity_usr_hash=PARTUUID=aaaa " +
				"systemd.verity_usr_options=hash-offset=123,panic-on-corruption usrhash=deadbeef " +
				"root=LABEL=ROOT rootflags=rw consoleblank=0",
			expectedSpecs: []UkiAddonSpec{
				{
					FileName: "vmlinuz-6.6.92.2-2.azl3.addon.efi",
					Cmdline:  "mount.usr=/dev/mapper/usr mount.usrflags=ro root=LABEL=ROOT rootflags=rw consoleblank=0",
				},
				{
					FileName: "verity.addon.efi",
					Cmdline: "systemd.verity_usr_data=PARTUUID=aaaa systemd.verity_usr_hash=PARTUUID=aaaa " +
						"systemd.verity_usr_options=hash-offset=123,panic-on-corruption usrhash=deadbeef",
				},
			},
		},
		{
			name: "verity and first-boot args both split out, persistent addon left clean",
			cmdline: "flatcar.first_boot=detected mount.usr=/dev/mapper/usr " +
				"systemd.verity_usr_hash=PARTUUID=bbbb usrhash=cafef00d root=LABEL=ROOT",
			expectedSpecs: []UkiAddonSpec{
				{FileName: "vmlinuz-6.6.92.2-2.azl3.addon.efi", Cmdline: "mount.usr=/dev/mapper/usr root=LABEL=ROOT"},
				{FileName: "verity.addon.efi", Cmdline: "systemd.verity_usr_hash=PARTUUID=bbbb usrhash=cafef00d"},
				{FileName: "firstboot.addon.efi", Cmdline: "flatcar.first_boot=detected"},
			},
		},
		{
			name:    "no verity args (verity disabled)",
			cmdline: "mount.usr=PARTUUID=aaaa mount.usrflags=ro root=LABEL=ROOT",
			expectedSpecs: []UkiAddonSpec{
				{FileName: "vmlinuz-6.6.92.2-2.azl3.addon.efi", Cmdline: "mount.usr=PARTUUID=aaaa mount.usrflags=ro root=LABEL=ROOT"},
			},
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
			specs, err := aclGetUkiAddonSpecs("vmlinuz-6.6.92.2-2.azl3", tt.cmdline)
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

// TestAclGetUkiAddonSpecsRoundTrip verifies that re-customization converges: the addons merge back
// in file-name order with the first-boot arg first, and re-splitting that cmdline yields identical
// specs.
func TestAclGetUkiAddonSpecsRoundTrip(t *testing.T) {
	kernel := "vmlinuz-6.6.92.2-2.azl3"

	specs, err := aclGetUkiAddonSpecs(kernel, "console=tty0 flatcar.first_boot=detected root=/dev/sda")
	assert.NoError(t, err)
	assert.Len(t, specs, 2)

	respecs, err := aclGetUkiAddonSpecs(kernel, "flatcar.first_boot=detected console=tty0 root=/dev/sda")
	assert.NoError(t, err)
	assert.Equal(t, specs, respecs)

	bootedSpecs, err := aclGetUkiAddonSpecs(kernel, "console=tty0 root=/dev/sda")
	assert.NoError(t, err)
	assert.Equal(t, specs[:1], bootedSpecs)
}

// TestAclGetUkiAddonSpecsVerityRoundTrip verifies the verity split also converges under
// re-customization, and that re-splitting the persistent addon alone (as read back by
// ExtractUkiAddonCmdline, which never looks at verity.addon.efi) never reintroduces verity args.
func TestAclGetUkiAddonSpecsVerityRoundTrip(t *testing.T) {
	kernel := "vmlinuz-6.6.92.2-2.azl3"
	cmdline := "mount.usr=/dev/mapper/usr systemd.verity_usr_hash=PARTUUID=aaaa usrhash=deadbeef root=LABEL=ROOT"

	specs, err := aclGetUkiAddonSpecs(kernel, cmdline)
	assert.NoError(t, err)
	assert.Len(t, specs, 2)
	persistent := specs[0]
	assert.Equal(t, "mount.usr=/dev/mapper/usr root=LABEL=ROOT", persistent.Cmdline)

	// Simulates Trident swapping verity.addon.efi to volume B and Image Customizer later reading
	// back ONLY the persistent addon's cmdline (its normal modify-mode read path) -- re-splitting
	// it must not resurrect any verity args, even though the original cmdline had them.
	respecs, err := aclGetUkiAddonSpecs(kernel, persistent.Cmdline)
	assert.NoError(t, err)
	assert.Equal(t, []UkiAddonSpec{persistent}, respecs)
}
