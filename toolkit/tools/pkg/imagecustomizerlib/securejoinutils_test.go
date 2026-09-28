// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecureJoinRootClampsSymlinkEscape(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	assert.NoError(t, os.MkdirAll(root, 0o755))

	// A directory OUTSIDE the root that a malicious/preserved symlink points at.
	hostArea := filepath.Join(t.TempDir(), "host")
	assert.NoError(t, os.MkdirAll(hostArea, 0o755))

	// Inside the root, an absolute symlink 'escape' -> hostArea, like a preserved
	// 'etc -> /' link that inject-files might resolve through.
	assert.NoError(t, os.Symlink(hostArea, filepath.Join(root, "escape")))

	// Writing through the link must be clamped back inside root, not hostArea.
	dest, err := secureJoinRoot(root, "/escape/payload.txt")
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(dest, root+string(os.PathSeparator)),
		"resolved destination (%s) must stay within root (%s)", dest, root)

	// Real callers create the destination directory before writing (file.Copy ->
	// CreateDestinationDir); do the same so the clamped write actually lands.
	assert.NoError(t, os.MkdirAll(filepath.Dir(dest), 0o755))
	assert.NoError(t, os.WriteFile(dest, []byte("payload"), 0o644))

	entries, err := os.ReadDir(hostArea)
	assert.NoError(t, err)
	assert.Empty(t, entries, "write must not escape root into %s", hostArea)

	_, err = os.Stat(dest)
	assert.NoError(t, err, "expected clamped write within root")
}
