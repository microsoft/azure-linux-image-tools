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

func TestAclDracutRegenerateArgsUsesAclConfDir(t *testing.T) {
	// The regenerated initramfs must include ACL's verity/storage dracut modules, which come from
	// ACL's config file /usr/share/distro/etc/dracut.conf.d/99-acl.conf. dracut's --confdir points
	// directly at the directory it globs *.conf files from (it does not descend into a dracut.conf.d
	// subdir), and dracut does not read that path by default (the image's /etc is empty), so
	// --confdir must point straight at ACL's dracut.conf.d.
	args := aclDracutRegenerateArgs()

	confdirIdx := slices.Index(args, "--confdir")
	require.GreaterOrEqual(t, confdirIdx, 0, "expected --confdir in dracut args")
	require.Less(t, confdirIdx+1, len(args), "expected a value after --confdir")

	assert.Equal(t, "/usr/share/distro/etc/dracut.conf.d", args[confdirIdx+1])
}

func TestAclFindBootPartitionUuidFromEspReturnsEmptyWithoutError(t *testing.T) {
	// ACL is systemd-boot with no grub.cfg on the ESP; the ESP itself is the boot partition.
	// FindBootPartitionUuidFromEsp must return an empty UUID with NO error so findBootPartitionFromEsp
	// treats the ESP as the boot partition. Returning an error here broke the output.artifacts extract
	// path on ACL.
	d := &aclDistroHandler{}
	uuid, err := d.FindBootPartitionUuidFromEsp("/nonexistent-esp")
	assert.NoError(t, err)
	assert.Empty(t, uuid)
}
