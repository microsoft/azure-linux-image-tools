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
				{FileName: "oem.addon.efi", Cmdline: "console=tty0 root=/dev/sda"},
				{FileName: "firstboot.addon.efi", Cmdline: "flatcar.first_boot=detected"},
			},
		},
		{
			name:    "duplicated first-boot args (first, last, adjacent)",
			cmdline: "flatcar.first_boot=detected console=tty0 flatcar.first_boot=detected flatcar.first_boot=detected",
			expectedSpecs: []UkiAddonSpec{
				{FileName: "oem.addon.efi", Cmdline: "console=tty0"},
				{FileName: "firstboot.addon.efi", Cmdline: "flatcar.first_boot=detected"},
			},
		},
		{
			name:    "no first-boot arg (already-booted image)",
			cmdline: "console=tty0 root=/dev/sda",
			expectedSpecs: []UkiAddonSpec{
				{FileName: "oem.addon.efi", Cmdline: "console=tty0 root=/dev/sda"},
			},
		},
		{
			name:    "extra whitespace collapsed",
			cmdline: "console=ttyS0,115200n8   flatcar.first_boot=detected  flatcar.oem.id=azure",
			expectedSpecs: []UkiAddonSpec{
				{FileName: "oem.addon.efi", Cmdline: "console=ttyS0,115200n8 flatcar.oem.id=azure"},
				{FileName: "firstboot.addon.efi", Cmdline: "flatcar.first_boot=detected"},
			},
		},
		{
			name: "similar args preserved",
			cmdline: "myflatcar.first_boot=detected flatcar.first_boot=1 flatcar.first_boot=detected2 " +
				"flatcar.first_boot=detected",
			expectedSpecs: []UkiAddonSpec{
				{
					FileName: "oem.addon.efi",
					Cmdline:  "myflatcar.first_boot=detected flatcar.first_boot=1 flatcar.first_boot=detected2",
				},
				{FileName: "firstboot.addon.efi", Cmdline: "flatcar.first_boot=detected"},
			},
		},
		{
			name:    "verity args split into their own addon",
			cmdline: "console=tty0 systemd.verity_usr_data=PARTUUID=aaaa systemd.verity_usr_hash=PARTUUID=bbbb usrhash=deadbeef root=/dev/sda flatcar.first_boot=detected",
			expectedSpecs: []UkiAddonSpec{
				{FileName: "oem.addon.efi", Cmdline: "console=tty0 root=/dev/sda"},
				{FileName: "verity.addon.efi", Cmdline: "systemd.verity_usr_data=PARTUUID=aaaa systemd.verity_usr_hash=PARTUUID=bbbb usrhash=deadbeef"},
				{FileName: "firstboot.addon.efi", Cmdline: "flatcar.first_boot=detected"},
			},
		},
		{
			name:    "verity args only, no oem args",
			cmdline: "systemd.verity_usr_hash=PARTUUID=bbbb usrhash=deadbeef",
			expectedSpecs: []UkiAddonSpec{
				{FileName: "verity.addon.efi", Cmdline: "systemd.verity_usr_hash=PARTUUID=bbbb usrhash=deadbeef"},
			},
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

// TestAclGetUkiAddonSpecsPreserving_NoExistingAddons verifies the nil/empty-existingAddons case
// falls back to the exact same behavior as aclGetUkiAddonSpecs.
func TestAclGetUkiAddonSpecsPreserving_NoExistingAddons(t *testing.T) {
	kernel := "vmlinuz-6.6.92.2-2.azl3"
	cmdline := "console=tty0 flatcar.first_boot=detected root=/dev/sda"

	expected, err := aclGetUkiAddonSpecs(kernel, cmdline)
	assert.NoError(t, err)

	specs, err := aclGetUkiAddonSpecsPreserving(kernel, cmdline, nil)
	assert.NoError(t, err)
	assert.Equal(t, expected, specs)
}

// TestAclGetUkiAddonSpecsPreserving_PassesThroughUnmanagedAddons verifies that addon files ACL
// doesn't manage (verity, fips, kdump, ...) are preserved as their own files untouched, instead of
// having their arguments folded into a generated addon -- this is the core fix for the ACL-T
// AB-update regression, where a stale verity.addon.efi's args got folded into a persistent
// "vmlinuz-<kernel>.addon.efi" bundle and then outlived Trident's per-slot verity.addon.efi swap.
// It also verifies that a legacy "vmlinuz-<kernel>.addon.efi" bundle left behind by an older Image
// Customizer version self-heals: its content is folded back into oem.addon.efi and the legacy file
// itself no longer appears in the output, matching a stock ACL image's native layout.
func TestAclGetUkiAddonSpecsPreserving_PassesThroughUnmanagedAddons(t *testing.T) {
	kernel := "vmlinuz-6.6.92.2-2.azl3"

	existingAddons := map[string]string{
		ukiMainCmdlineAddonKey:              "mount.usr=/dev/mapper/usr root=LABEL=ROOT rootflags=rw",
		"oem.addon.efi":                     "flatcar.oem.id=azure console=ttyS0",
		"verity.addon.efi":                  "systemd.verity_usr_hash=PARTUUID=aaaa usrhash=deadbeef",
		"vmlinuz-6.6.92.2-2.azl3.addon.efi": "consoleblank=0",
		"firstboot.addon.efi":               "flatcar.first_boot=detected",
	}

	// The flattened cmdline is what extractCmdlineFromUkiWithObjcopy would have produced -- unused
	// by aclGetUkiAddonSpecsPreserving here since existingAddons is populated, but kept realistic.
	flattenedCmdline := "mount.usr=/dev/mapper/usr root=LABEL=ROOT rootflags=rw consoleblank=0 " +
		"flatcar.first_boot=detected flatcar.oem.id=azure console=ttyS0 " +
		"systemd.verity_usr_hash=PARTUUID=aaaa usrhash=deadbeef"

	specs, err := aclGetUkiAddonSpecsPreserving(kernel, flattenedCmdline, existingAddons)
	assert.NoError(t, err)

	// "oem.addon.efi" (managed) absorbs the legacy "vmlinuz-<kernel>.addon.efi" bundle's content
	// (consoleblank=0); "verity.addon.efi" (unmanaged) passes through untouched; no
	// "vmlinuz-<kernel>.addon.efi" file remains.
	assert.Equal(t, []UkiAddonSpec{
		{FileName: "firstboot.addon.efi", Cmdline: "flatcar.first_boot=detected"},
		{FileName: "oem.addon.efi", Cmdline: "mount.usr=/dev/mapper/usr root=LABEL=ROOT rootflags=rw flatcar.oem.id=azure console=ttyS0 consoleblank=0"},
		{FileName: "verity.addon.efi", Cmdline: "systemd.verity_usr_hash=PARTUUID=aaaa usrhash=deadbeef"},
	}, specs)
}

// TestAclGetUkiAddonSpecsPreserving_VeritySwapSurvives simulates the exact regression scenario:
// Trident swaps verity.addon.efi to a new (volume B) value between two runs of Image Customizer
// re-customization; the oem addon it manages must not change as a result, and the new
// verity.addon.efi content must be preserved verbatim rather than overridden.
func TestAclGetUkiAddonSpecsPreserving_VeritySwapSurvives(t *testing.T) {
	kernel := "vmlinuz-6.6.92.2-2.azl3"

	volumeAAddons := map[string]string{
		ukiMainCmdlineAddonKey: "mount.usr=/dev/mapper/usr root=LABEL=ROOT",
		"verity.addon.efi":     "systemd.verity_usr_hash=PARTUUID=aaaa usrhash=volumeA",
	}
	volumeBAddons := map[string]string{
		ukiMainCmdlineAddonKey: "mount.usr=/dev/mapper/usr root=LABEL=ROOT",
		"verity.addon.efi":     "systemd.verity_usr_hash=PARTUUID=bbbb usrhash=volumeB",
	}

	specsA, err := aclGetUkiAddonSpecsPreserving(kernel, "unused", volumeAAddons)
	assert.NoError(t, err)
	specsB, err := aclGetUkiAddonSpecsPreserving(kernel, "unused", volumeBAddons)
	assert.NoError(t, err)

	// The oem addon is identical in both cases -- verity content never leaked into it.
	persistentA := mustFindAddonSpec(t, specsA, "oem.addon.efi")
	persistentB := mustFindAddonSpec(t, specsB, "oem.addon.efi")
	assert.Equal(t, persistentA, persistentB)

	// The verity addon is passed through with its own (volume-specific) content, unmodified.
	assert.Equal(t, "systemd.verity_usr_hash=PARTUUID=aaaa usrhash=volumeA", mustFindAddonSpec(t, specsA, "verity.addon.efi").Cmdline)
	assert.Equal(t, "systemd.verity_usr_hash=PARTUUID=bbbb usrhash=volumeB", mustFindAddonSpec(t, specsB, "verity.addon.efi").Cmdline)
}

func mustFindAddonSpec(t *testing.T, specs []UkiAddonSpec, fileName string) UkiAddonSpec {
	t.Helper()
	for _, spec := range specs {
		if spec.FileName == fileName {
			return spec
		}
	}
	t.Fatalf("no addon spec found with file name %q", fileName)
	return UkiAddonSpec{}
}
