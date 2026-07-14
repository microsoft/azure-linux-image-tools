// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAclDracutRegenerateArgsUsesExplicitTmpdir(t *testing.T) {
	args := aclDracutRegenerateArgs()

	// Regenerate all initramfs images.
	assert.Contains(t, args, "--force")
	assert.Contains(t, args, "--regenerate-all")

	// dracut's staging dir must be pinned to an explicit, xattr-capable path (not the default
	// /var/tmp, which may be shadowed by a mount that rejects security.* xattrs).
	tmpdirIdx := slices.Index(args, "--tmpdir")
	require.GreaterOrEqual(t, tmpdirIdx, 0, "expected --tmpdir in dracut args")
	require.Less(t, tmpdirIdx+1, len(args), "expected a value after --tmpdir")

	tmpdir := args[tmpdirIdx+1]
	assert.Equal(t, "/"+aclDracutTmpDirName, tmpdir)
	assert.NotEqual(t, "/var/tmp", tmpdir, "dracut tmpdir must not be the default /var/tmp")
}
