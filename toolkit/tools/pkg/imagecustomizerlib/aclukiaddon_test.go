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

// TestAclGetUkiAddonSpecsPreserving_PassesThroughExistingAddonsVerbatim verifies that when
// cmdline matches (is fully accounted for by) the existing addon structure, every existing addon
// file -- including ones ACL doesn't specifically know about (fips, kdump, a legacy
// "vmlinuz-<kernel>.addon.efi" bundle, ...) -- passes through as its own file, byte-for-byte
// unmodified, and no customized.addon.efi is created. This is the core fix for the ACL-T AB-update
// regression: verity.addon.efi in particular must never have its arguments folded into (or
// overridden by) any other addon file, since Trident regenerates it independently per A/B slot.
func TestAclGetUkiAddonSpecsPreserving_PassesThroughExistingAddonsVerbatim(t *testing.T) {
	kernel := "vmlinuz-6.6.92.2-2.azl3"

	existingAddons := map[string]string{
		ukiMainCmdlineAddonKey:              "mount.usr=/dev/mapper/usr root=LABEL=ROOT rootflags=rw",
		"oem.addon.efi":                     "flatcar.oem.id=azure console=ttyS0",
		"verity.addon.efi":                  "systemd.verity_usr_hash=PARTUUID=aaaa usrhash=deadbeef",
		"vmlinuz-6.6.92.2-2.azl3.addon.efi": "consoleblank=0",
		"firstboot.addon.efi":               "flatcar.first_boot=detected",
	}

	// The flattened cmdline is what extractCmdlineFromUkiWithObjcopy would have produced -- every
	// argument is already accounted for by some existing addon file, so nothing should change.
	flattenedCmdline := "mount.usr=/dev/mapper/usr root=LABEL=ROOT rootflags=rw consoleblank=0 " +
		"flatcar.first_boot=detected flatcar.oem.id=azure console=ttyS0 " +
		"systemd.verity_usr_hash=PARTUUID=aaaa usrhash=deadbeef"

	specs, err := aclGetUkiAddonSpecsPreserving(kernel, flattenedCmdline, existingAddons)
	assert.NoError(t, err)

	// Every non-main existing addon file passes through untouched -- including the legacy
	// "vmlinuz-<kernel>.addon.efi" bundle, which is no longer specially self-healed. No
	// customized.addon.efi is created since there's nothing new/changed.
	assert.Equal(t, []UkiAddonSpec{
		{FileName: "firstboot.addon.efi", Cmdline: "flatcar.first_boot=detected"},
		{FileName: "oem.addon.efi", Cmdline: "flatcar.oem.id=azure console=ttyS0"},
		{FileName: "verity.addon.efi", Cmdline: "systemd.verity_usr_hash=PARTUUID=aaaa usrhash=deadbeef"},
		{FileName: "vmlinuz-6.6.92.2-2.azl3.addon.efi", Cmdline: "consoleblank=0"},
	}, specs)
}

// TestAclGetUkiAddonSpecsPreserving_VeritySwapSurvives simulates the exact regression scenario:
// Trident swaps verity.addon.efi to a new (volume B) value between two runs of Image Customizer
// re-customization; every other existing addon must not change as a result, and the new
// verity.addon.efi content must be preserved verbatim rather than overridden.
func TestAclGetUkiAddonSpecsPreserving_VeritySwapSurvives(t *testing.T) {
	kernel := "vmlinuz-6.6.92.2-2.azl3"

	volumeAAddons := map[string]string{
		ukiMainCmdlineAddonKey: "mount.usr=/dev/mapper/usr root=LABEL=ROOT",
		"oem.addon.efi":        "flatcar.oem.id=azure",
		"verity.addon.efi":     "systemd.verity_usr_hash=PARTUUID=aaaa usrhash=volumeA",
	}
	volumeBAddons := map[string]string{
		ukiMainCmdlineAddonKey: "mount.usr=/dev/mapper/usr root=LABEL=ROOT",
		"oem.addon.efi":        "flatcar.oem.id=azure",
		"verity.addon.efi":     "systemd.verity_usr_hash=PARTUUID=bbbb usrhash=volumeB",
	}
	// cmdline exactly matches the existing structure in both cases -- nothing new/changed.
	fullCmdlineA := "mount.usr=/dev/mapper/usr root=LABEL=ROOT flatcar.oem.id=azure " +
		"systemd.verity_usr_hash=PARTUUID=aaaa usrhash=volumeA"
	fullCmdlineB := "mount.usr=/dev/mapper/usr root=LABEL=ROOT flatcar.oem.id=azure " +
		"systemd.verity_usr_hash=PARTUUID=bbbb usrhash=volumeB"

	specsA, err := aclGetUkiAddonSpecsPreserving(kernel, fullCmdlineA, volumeAAddons)
	assert.NoError(t, err)
	specsB, err := aclGetUkiAddonSpecsPreserving(kernel, fullCmdlineB, volumeBAddons)
	assert.NoError(t, err)

	// The oem addon is identical in both cases -- verity content never leaked into it.
	oemA := mustFindAddonSpec(t, specsA, "oem.addon.efi")
	oemB := mustFindAddonSpec(t, specsB, "oem.addon.efi")
	assert.Equal(t, oemA, oemB)

	// No customized.addon.efi is created, since nothing new/changed relative to existing addons.
	for _, spec := range append(specsA, specsB...) {
		assert.NotEqual(t, aclCustomizedAddonName, spec.FileName)
	}

	// The verity addon is passed through with its own (volume-specific) content, unmodified.
	assert.Equal(t, "systemd.verity_usr_hash=PARTUUID=aaaa usrhash=volumeA", mustFindAddonSpec(t, specsA, "verity.addon.efi").Cmdline)
	assert.Equal(t, "systemd.verity_usr_hash=PARTUUID=bbbb usrhash=volumeB", mustFindAddonSpec(t, specsB, "verity.addon.efi").Cmdline)
}

// TestAclGetUkiAddonSpecsPreserving_NewArgGoesToCustomizedAddon verifies that a kernel
// command-line argument added by user configuration (e.g. os.uki.kernelCommandLine), which isn't
// already covered by any existing addon file and doesn't collide with one, is written to its own
// dedicated customized.addon.efi rather than being folded into an existing native addon file.
func TestAclGetUkiAddonSpecsPreserving_NewArgGoesToCustomizedAddon(t *testing.T) {
	kernel := "vmlinuz-6.6.92.2-2.azl3"

	existingAddons := map[string]string{
		ukiMainCmdlineAddonKey: "mount.usr=/dev/mapper/usr root=LABEL=ROOT",
		"oem.addon.efi":        "flatcar.oem.id=azure",
	}
	cmdline := "mount.usr=/dev/mapper/usr root=LABEL=ROOT flatcar.oem.id=azure mitigations=off"

	specs, err := aclGetUkiAddonSpecsPreserving(kernel, cmdline, existingAddons)
	assert.NoError(t, err)

	assert.Equal(t, "flatcar.oem.id=azure", mustFindAddonSpec(t, specs, "oem.addon.efi").Cmdline)
	assert.Equal(t, "mitigations=off", mustFindAddonSpec(t, specs, aclCustomizedAddonName).Cmdline)
}

// TestAclGetUkiAddonSpecsPreserving_CustomizedAddonUpdatedInPlace verifies that changing the
// value of an argument already carried in an existing customized.addon.efi updates that same
// argument in place (preserving its position and every other existing customized argument),
// rather than duplicating it or erroring out.
func TestAclGetUkiAddonSpecsPreserving_CustomizedAddonUpdatedInPlace(t *testing.T) {
	kernel := "vmlinuz-6.6.92.2-2.azl3"

	existingAddons := map[string]string{
		ukiMainCmdlineAddonKey: "mount.usr=/dev/mapper/usr root=LABEL=ROOT",
		aclCustomizedAddonName: "mitigations=off panic=30",
	}
	// mitigations= is updated; panic= is unchanged.
	cmdline := "mount.usr=/dev/mapper/usr root=LABEL=ROOT mitigations=auto panic=30"

	specs, err := aclGetUkiAddonSpecsPreserving(kernel, cmdline, existingAddons)
	assert.NoError(t, err)

	assert.Equal(t, "mitigations=auto panic=30", mustFindAddonSpec(t, specs, aclCustomizedAddonName).Cmdline)
}

// TestAclGetUkiAddonSpecsPreserving_NewVerityArgIgnored verifies that a dm-verity-named argument
// added via cmdline (not already present in any existing addon) is silently ignored rather than
// being written into customized.addon.efi: Trident and Image Customizer's own verity-hash refresh
// (aclUpdateVerityAddonTemplates) already own that data exclusively.
func TestAclGetUkiAddonSpecsPreserving_NewVerityArgIgnored(t *testing.T) {
	kernel := "vmlinuz-6.6.92.2-2.azl3"

	existingAddons := map[string]string{
		ukiMainCmdlineAddonKey: "mount.usr=/dev/mapper/usr root=LABEL=ROOT",
	}
	cmdline := "mount.usr=/dev/mapper/usr root=LABEL=ROOT usrhash=deadbeef"

	specs, err := aclGetUkiAddonSpecsPreserving(kernel, cmdline, existingAddons)
	assert.NoError(t, err)

	assert.Empty(t, specs)
}

// TestAclGetUkiAddonSpecsPreserving_OverrideOfExistingAddonArgErrors verifies that changing the
// value of an argument already owned by an existing (non-customized) addon file is rejected: since
// addons load in a fixed alphabetical order, such an override cannot be reliably expressed.
func TestAclGetUkiAddonSpecsPreserving_OverrideOfExistingAddonArgErrors(t *testing.T) {
	kernel := "vmlinuz-6.6.92.2-2.azl3"

	existingAddons := map[string]string{
		ukiMainCmdlineAddonKey: "mount.usr=/dev/mapper/usr root=LABEL=ROOT",
		"oem.addon.efi":        "console=tty0",
	}
	cmdline := "mount.usr=/dev/mapper/usr root=LABEL=ROOT console=ttyS0"

	_, err := aclGetUkiAddonSpecsPreserving(kernel, cmdline, existingAddons)
	assert.ErrorIs(t, err, ErrAclUkiAddonSplit)
	assert.ErrorContains(t, err, "console=ttyS0")
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
