// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The base image ships a single UKI (for the stock kernel) whose cmdline carries the essential ACL
// boot args, including the /usr dm-verity args. On ACL it is keyed by the UKI filename stem.
const aclBaseUkiCmdline = "root=PARTUUID=abc rd.systemd.verity=1 usrhash=deadbeef " +
	"systemd.verity_usr_data=PARTUUID=usr systemd.verity_usr_hash=PARTUUID=usr " +
	"systemd.verity_usr_options=panic-on-corruption,hash-offset=1073741824 console=tty0"

func TestInheritedBaseCmdlineSingleUki(t *testing.T) {
	// ACL: a single base UKI cmdline (keyed "acl").
	kernelToArgs := map[string]string{
		"acl": aclBaseUkiCmdline,
	}
	cmdline, ok := inheritedBaseCmdline(kernelToArgs)
	require.True(t, ok)
	assert.Equal(t, aclBaseUkiCmdline, cmdline)
}

func TestInheritedBaseCmdlineNone(t *testing.T) {
	cmdline, ok := inheritedBaseCmdline(map[string]string{})
	assert.False(t, ok)
	assert.Empty(t, cmdline)
}

func TestInheritedBaseCmdlineAmbiguous(t *testing.T) {
	// Multiple distinct base cmdlines: do not guess.
	kernelToArgs := map[string]string{
		"vmlinuz-a": "root=PARTUUID=a console=tty0",
		"vmlinuz-b": "root=PARTUUID=b console=ttyS0",
	}
	_, ok := inheritedBaseCmdline(kernelToArgs)
	assert.False(t, ok)
}

func TestSelectKernelBaseCmdlineKernelSwapInheritsBaseUki(t *testing.T) {
	// The base UKI (and thus kernelToArgs) is keyed by the removed stock kernel; the swapped-in
	// kernel-hwe has no per-kernel cmdline, so it must inherit the base UKI cmdline.
	const newKernel = "vmlinuz-6.18.36.1-1.azl3"
	kernelToArgs := map[string]string{
		"acl": aclBaseUkiCmdline,
	}
	fallback, hasFallback := inheritedBaseCmdline(kernelToArgs)
	require.True(t, hasFallback)

	cmdline, err := selectKernelBaseCmdline(newKernel, nil /*existingKernelInfo*/, kernelToArgs,
		fallback, hasFallback)
	require.NoError(t, err)
	assert.Equal(t, aclBaseUkiCmdline, cmdline)
}

func TestSelectKernelBaseCmdlinePrefersExistingThenArgs(t *testing.T) {
	existing := map[string]UkiKernelInfo{
		"vmlinuz-1": {Cmdline: "from-existing"},
	}
	kernelToArgs := map[string]string{
		"vmlinuz-2": "from-args",
	}

	// Existing per-kernel cmdline wins.
	c1, err := selectKernelBaseCmdline("vmlinuz-1", existing, kernelToArgs, "fallback", true)
	require.NoError(t, err)
	assert.Equal(t, "from-existing", c1)

	// Otherwise the per-kernel extracted args are used (not the fallback).
	c2, err := selectKernelBaseCmdline("vmlinuz-2", existing, kernelToArgs, "fallback", true)
	require.NoError(t, err)
	assert.Equal(t, "from-args", c2)
}

func TestSelectKernelBaseCmdlineNoFallbackErrors(t *testing.T) {
	_, err := selectKernelBaseCmdline("vmlinuz-x", nil, map[string]string{}, "", false /*hasFallback*/)
	require.Error(t, err)
	assert.ErrorContains(t, err, "no command line arguments found for kernel")
}

func TestApplyAclOemIdStripsFlatcarAndCoreos(t *testing.T) {
	// flatcar spelling present -> replaced with exactly one flatcar.oem.id, azure token gone.
	in := "root=PARTUUID=abc flatcar.oem.id=azure console=tty0"
	out := applyAclOemId(in, "metal")
	assert.Contains(t, out, "flatcar.oem.id=metal")
	assert.NotContains(t, out, "flatcar.oem.id=azure")
	assert.NotContains(t, out, "oem.id=azure")
	assert.Equal(t, 1, countTokenPrefix(out, "flatcar.oem.id="))
	// Other args preserved.
	assert.Contains(t, out, "root=PARTUUID=abc")
	assert.Contains(t, out, "console=tty0")
}

func TestApplyAclOemIdStripsLegacyCoreosSpelling(t *testing.T) {
	in := "root=PARTUUID=abc coreos.oem.id=azure console=tty0"
	out := applyAclOemId(in, "metal")
	assert.Contains(t, out, "flatcar.oem.id=metal")
	assert.NotContains(t, out, "coreos.oem.id=azure")
	// Only the modern flatcar spelling is written, not coreos.
	assert.Equal(t, 0, countTokenPrefix(out, "coreos.oem.id="))
	assert.Equal(t, 1, countTokenPrefix(out, "flatcar.oem.id="))
}

func TestApplyAclOemIdWhenNonepresent(t *testing.T) {
	in := "root=PARTUUID=abc console=tty0"
	out := applyAclOemId(in, "metal")
	assert.Equal(t, 1, countTokenPrefix(out, "flatcar.oem.id="))
	assert.Contains(t, out, "flatcar.oem.id=metal")
}

func TestApplyAclOemIdIdempotent(t *testing.T) {
	in := "root=PARTUUID=abc flatcar.oem.id=azure console=tty0"
	once := applyAclOemId(in, "metal")
	twice := applyAclOemId(once, "metal")
	assert.Equal(t, once, twice)
	assert.Equal(t, 1, countTokenPrefix(twice, "flatcar.oem.id="))
}

func TestApplyAclOemIdMultipleExistingTokens(t *testing.T) {
	// Both spellings and duplicates present -> all stripped, exactly one flatcar.oem.id remains.
	in := "flatcar.oem.id=azure root=x coreos.oem.id=azure flatcar.oem.id=azure console=tty0"
	out := applyAclOemId(in, "metal")
	assert.Equal(t, 1, countTokenPrefix(out, "flatcar.oem.id="))
	assert.Equal(t, 0, countTokenPrefix(out, "coreos.oem.id="))
	assert.NotContains(t, out, "=azure")
}

func countTokenPrefix(cmdline string, prefix string) int {
	n := 0
	for _, tok := range strings.Fields(cmdline) {
		if strings.HasPrefix(tok, prefix) {
			n++
		}
	}
	return n
}
