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

func TestAclEtcOverlayOptions(t *testing.T) {
	// The scoped /etc overlay must layer ACL's factory /etc (lower) under the image's /etc (upper),
	// with the work dir on the same filesystem as the upper (the image ROOT).
	opts := aclEtcOverlayOptions(
		"/mnt/imageroot/usr/share/distro/etc",
		"/mnt/imageroot/etc",
		"/mnt/imageroot/.ic-etc-overlay-work",
	)

	assert.Contains(t, opts, "lowerdir=/mnt/imageroot/usr/share/distro/etc")
	assert.Contains(t, opts, "upperdir=/mnt/imageroot/etc")
	assert.Contains(t, opts, "workdir=/mnt/imageroot/.ic-etc-overlay-work")
	assert.Contains(t, opts, "redirect_dir=on")
	assert.Contains(t, opts, "metacopy=off")
}

func TestAclFactoryEtcDirIsDistroEtc(t *testing.T) {
	// The lowerdir must be ACL's factory /etc (the source of 99-acl.conf).
	assert.Equal(t, "usr/share/distro/etc", aclFactoryEtcDir)
}
