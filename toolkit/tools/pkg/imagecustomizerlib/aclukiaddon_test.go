// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAclGetUkiAddonSpecsPreserving_NoExistingAddons verifies that when there's no existing addon
// structure at all to preserve (existingAddons is nil), every argument in cmdline is treated as
// new: verity-named arguments are rejected, and every other argument is written into a single
// aclCustomizedAddonName -- no oem/verity/firstboot split is invented, since there's no input
// structure to justify one. (For ACL this case should never actually occur in practice: every ACL
// kernel ships with existing UKI structure.)
func TestAclGetUkiAddonSpecsPreserving_NoExistingAddons(t *testing.T) {
	kernel := "vmlinuz-6.6.92.2-2.azl3"
	cmdline := "console=tty0 flatcar.first_boot=detected root=/dev/sda"

	specs, err := aclGetUkiAddonSpecsPreserving(kernel, cmdline, nil)
	assert.NoError(t, err)

	assert.Equal(t, []UkiAddonSpec{
		{FileName: aclCustomizedAddonName, Cmdline: "console=tty0 flatcar.first_boot=detected root=/dev/sda"},
	}, specs)
}

// TestAclGetUkiAddonSpecsPreserving_NoExistingAddonsVerityRejected verifies that even with no
// existing structure at all, a verity-named argument is still rejected rather than silently
// written into aclCustomizedAddonName.
func TestAclGetUkiAddonSpecsPreserving_NoExistingAddonsVerityRejected(t *testing.T) {
	kernel := "vmlinuz-6.6.92.2-2.azl3"
	cmdline := "console=tty0 usrhash=deadbeef"

	_, err := aclGetUkiAddonSpecsPreserving(kernel, cmdline, nil)
	assert.ErrorIs(t, err, ErrAclUkiAddonSplit)
	assert.ErrorContains(t, err, "usrhash=deadbeef")
}

// TestAclGetUkiAddonSpecsPreserving_PassesThroughExistingAddonsVerbatim verifies that when
// cmdline matches (is fully accounted for by) the existing addon structure, every existing addon
// file -- including ones ACL doesn't specifically know about (fips, kdump, ...) -- passes through
// as its own file, byte-for-byte unmodified, and no customized.addon.efi is created. This is the
// core fix for the ACL-T AB-update regression: verity.addon.efi in particular must never have its
// arguments folded into (or overridden by) any other addon file, since Trident regenerates it
// independently per A/B slot.
func TestAclGetUkiAddonSpecsPreserving_PassesThroughExistingAddonsVerbatim(t *testing.T) {
	kernel := "vmlinuz-6.6.92.2-2.azl3"

	existingAddons := map[string]string{
		"oem.addon.efi":       "flatcar.oem.id=azure console=ttyS0",
		"verity.addon.efi":    "systemd.verity_usr_hash=PARTUUID=aaaa usrhash=deadbeef",
		"firstboot.addon.efi": "flatcar.first_boot=detected",
	}

	// The flattened cmdline is what extractCmdlineFromUkiWithObjcopy would have produced -- every
	// argument is already accounted for by some existing addon file, so nothing should change.
	flattenedCmdline := "flatcar.first_boot=detected flatcar.oem.id=azure console=ttyS0 " +
		"systemd.verity_usr_hash=PARTUUID=aaaa usrhash=deadbeef"

	specs, err := aclGetUkiAddonSpecsPreserving(kernel, flattenedCmdline, existingAddons)
	assert.NoError(t, err)

	// Every existing addon file passes through untouched. No customized.addon.efi is created since
	// there's nothing new/changed.
	assert.Equal(t, []UkiAddonSpec{
		{FileName: "firstboot.addon.efi", Cmdline: "flatcar.first_boot=detected"},
		{FileName: "oem.addon.efi", Cmdline: "flatcar.oem.id=azure console=ttyS0"},
		{FileName: "verity.addon.efi", Cmdline: "systemd.verity_usr_hash=PARTUUID=aaaa usrhash=deadbeef"},
	}, specs)
}

// TestAclGetUkiAddonSpecsPreserving_MainCmdlinePreservedInVmlinuzAddon verifies that whatever
// cmdline the input image's main UKI carried directly in its own .cmdline section is re-homed into
// its own addon file (ukiAddonFileName(kernel)) rather than silently dropped -- Image Customizer's
// UKI build never re-embeds a cmdline into the rebuilt main UKI itself.
func TestAclGetUkiAddonSpecsPreserving_MainCmdlinePreservedInVmlinuzAddon(t *testing.T) {
	kernel := "vmlinuz-6.6.92.2-2.azl3"

	existingAddons := map[string]string{
		ukiMainCmdlineAddonKey: "mount.usr=/dev/mapper/usr root=LABEL=ROOT rootflags=rw consoleblank=0",
		"oem.addon.efi":        "flatcar.oem.id=azure",
	}
	// cmdline exactly matches the existing structure -- nothing new/changed.
	cmdline := "mount.usr=/dev/mapper/usr root=LABEL=ROOT rootflags=rw consoleblank=0 flatcar.oem.id=azure"

	specs, err := aclGetUkiAddonSpecsPreserving(kernel, cmdline, existingAddons)
	assert.NoError(t, err)

	assert.Equal(t, []UkiAddonSpec{
		{FileName: "oem.addon.efi", Cmdline: "flatcar.oem.id=azure"},
		{
			FileName: "vmlinuz-6.6.92.2-2.azl3.addon.efi",
			Cmdline:  "mount.usr=/dev/mapper/usr root=LABEL=ROOT rootflags=rw consoleblank=0",
		},
	}, specs)
}

// TestAclGetUkiAddonSpecsPreserving_MainCmdlineMergesWithExistingSameNamedAddon verifies that if
// the input image happens to already have both a main-embedded cmdline AND a separate addon file
// that happens to share the exact output name Image Customizer would otherwise invent for the
// preserved main cmdline (ukiAddonFileName(kernel)), the two are merged into that single file
// rather than one silently overwriting the other.
func TestAclGetUkiAddonSpecsPreserving_MainCmdlineMergesWithExistingSameNamedAddon(t *testing.T) {
	kernel := "vmlinuz-6.6.92.2-2.azl3"

	existingAddons := map[string]string{
		ukiMainCmdlineAddonKey:              "mount.usr=/dev/mapper/usr root=LABEL=ROOT",
		"vmlinuz-6.6.92.2-2.azl3.addon.efi": "consoleblank=0",
	}
	cmdline := "mount.usr=/dev/mapper/usr root=LABEL=ROOT consoleblank=0"

	specs, err := aclGetUkiAddonSpecsPreserving(kernel, cmdline, existingAddons)
	assert.NoError(t, err)

	assert.Equal(t, []UkiAddonSpec{
		{
			FileName: "vmlinuz-6.6.92.2-2.azl3.addon.efi",
			Cmdline:  "mount.usr=/dev/mapper/usr root=LABEL=ROOT consoleblank=0",
		},
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

// TestAclGetUkiAddonSpecsPreserving_NewVerityArgErrors verifies that a dm-verity-named argument
// added via cmdline (not already present in any existing addon) is rejected with an error rather
// than being written into customized.addon.efi or silently ignored: Trident and Image Customizer's
// own verity-hash refresh (aclUpdateVerityAddonTemplates) already own that data exclusively, and
// kernel command-line customization is never a supported way to set it.
func TestAclGetUkiAddonSpecsPreserving_NewVerityArgErrors(t *testing.T) {
	kernel := "vmlinuz-6.6.92.2-2.azl3"

	existingAddons := map[string]string{
		ukiMainCmdlineAddonKey: "mount.usr=/dev/mapper/usr root=LABEL=ROOT",
	}
	cmdline := "mount.usr=/dev/mapper/usr root=LABEL=ROOT usrhash=deadbeef"

	_, err := aclGetUkiAddonSpecsPreserving(kernel, cmdline, existingAddons)
	assert.ErrorIs(t, err, ErrAclUkiAddonSplit)
	assert.ErrorContains(t, err, "usrhash=deadbeef")
}

// TestAclGetUkiAddonSpecsPreserving_ChangedVerityArgErrors verifies that CHANGING the value of an
// argument already owned by an existing verity addon file is also rejected -- not just a brand-new
// verity-named argument -- since verity settings can never be a supported target for kernel
// command-line customization, regardless of whether they previously existed.
func TestAclGetUkiAddonSpecsPreserving_ChangedVerityArgErrors(t *testing.T) {
	kernel := "vmlinuz-6.6.92.2-2.azl3"

	existingAddons := map[string]string{
		ukiMainCmdlineAddonKey: "mount.usr=/dev/mapper/usr root=LABEL=ROOT",
		"verity.addon.efi":     "usrhash=deadbeef",
	}
	cmdline := "mount.usr=/dev/mapper/usr root=LABEL=ROOT usrhash=beefdead"

	_, err := aclGetUkiAddonSpecsPreserving(kernel, cmdline, existingAddons)
	assert.ErrorIs(t, err, ErrAclUkiAddonSplit)
	assert.ErrorContains(t, err, "usrhash=beefdead")
}

// TestAclGetUkiAddonSpecsPreserving_OverrideOfExistingAddonArgStripsAndMoves verifies that
// changing the value of an argument already owned by an existing (non-verity) addon file strips
// the stale copy from that file and moves the new value into customized.addon.efi, rather than
// erroring out: the owning file keeps every argument it's not being overridden on.
func TestAclGetUkiAddonSpecsPreserving_OverrideOfExistingAddonArgStripsAndMoves(t *testing.T) {
	kernel := "vmlinuz-6.6.92.2-2.azl3"

	existingAddons := map[string]string{
		ukiMainCmdlineAddonKey: "mount.usr=/dev/mapper/usr root=LABEL=ROOT",
		"oem.addon.efi":        "console=tty0 flatcar.oem.id=azure",
	}
	cmdline := "mount.usr=/dev/mapper/usr root=LABEL=ROOT console=ttyS0 flatcar.oem.id=azure"

	specs, err := aclGetUkiAddonSpecsPreserving(kernel, cmdline, existingAddons)
	assert.NoError(t, err)

	// console= was stripped out of oem.addon.efi; flatcar.oem.id= (unaffected) remains.
	assert.Equal(t, "flatcar.oem.id=azure", mustFindAddonSpec(t, specs, "oem.addon.efi").Cmdline)
	// The new console= value lives only in customized.addon.efi.
	assert.Equal(t, "console=ttyS0", mustFindAddonSpec(t, specs, aclCustomizedAddonName).Cmdline)
}

// TestAclGetUkiAddonSpecsPreserving_OverrideEmptiesOwningFile verifies that if stripping a
// conflicting argument leaves an existing addon file with no arguments at all, that file is
// omitted from the result entirely rather than emitted as an empty addon.
func TestAclGetUkiAddonSpecsPreserving_OverrideEmptiesOwningFile(t *testing.T) {
	kernel := "vmlinuz-6.6.92.2-2.azl3"

	existingAddons := map[string]string{
		ukiMainCmdlineAddonKey: "mount.usr=/dev/mapper/usr root=LABEL=ROOT",
		"oem.addon.efi":        "console=tty0",
	}
	cmdline := "mount.usr=/dev/mapper/usr root=LABEL=ROOT console=ttyS0"

	specs, err := aclGetUkiAddonSpecsPreserving(kernel, cmdline, existingAddons)
	assert.NoError(t, err)

	for _, spec := range specs {
		assert.NotEqual(t, "oem.addon.efi", spec.FileName)
	}
	assert.Equal(t, "console=ttyS0", mustFindAddonSpec(t, specs, aclCustomizedAddonName).Cmdline)
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
