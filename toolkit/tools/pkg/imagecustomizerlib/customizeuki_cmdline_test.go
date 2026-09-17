// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
