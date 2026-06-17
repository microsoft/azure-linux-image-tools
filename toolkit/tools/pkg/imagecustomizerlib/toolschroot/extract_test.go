// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package toolschroot

import (
	"archive/tar"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type tarEntry struct {
	header tar.Header
	body   []byte
}

func writeTar(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for i := range entries {
		hdr := entries[i].header
		if hdr.Typeflag == tar.TypeReg {
			hdr.Size = int64(len(entries[i].body))
		}
		require.NoError(t, tw.WriteHeader(&hdr))
		if len(entries[i].body) > 0 {
			_, err := tw.Write(entries[i].body)
			require.NoError(t, err)
		}
	}
	require.NoError(t, tw.Close())
	return buf.Bytes()
}

func TestExtractRegularFileDirAndSymlink(t *testing.T) {
	destDir := t.TempDir()

	data := writeTar(t, []tarEntry{
		{header: tar.Header{Name: "etc/", Typeflag: tar.TypeDir, Mode: 0o755}},
		{header: tar.Header{Name: "etc/hosts", Typeflag: tar.TypeReg, Mode: 0o644}, body: []byte("127.0.0.1 localhost\n")},
		{header: tar.Header{Name: "etc/aliases", Typeflag: tar.TypeSymlink, Linkname: "hosts"}},
	})

	require.NoError(t, extractTarLayer(bytes.NewReader(data), destDir))

	info, err := os.Stat(filepath.Join(destDir, "etc"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	content, err := os.ReadFile(filepath.Join(destDir, "etc/hosts"))
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1 localhost\n", string(content))

	linkTarget, err := os.Readlink(filepath.Join(destDir, "etc/aliases"))
	require.NoError(t, err)
	assert.Equal(t, "hosts", linkTarget)
}

func TestExtractWhiteoutDeletesExistingPath(t *testing.T) {
	destDir := t.TempDir()

	first := writeTar(t, []tarEntry{
		{header: tar.Header{Name: "etc/", Typeflag: tar.TypeDir, Mode: 0o755}},
		{header: tar.Header{Name: "etc/hosts", Typeflag: tar.TypeReg, Mode: 0o644}, body: []byte("v1")},
	})
	require.NoError(t, extractTarLayer(bytes.NewReader(first), destDir))

	second := writeTar(t, []tarEntry{
		{header: tar.Header{Name: "etc/.wh.hosts", Typeflag: tar.TypeReg, Mode: 0o644}},
	})
	require.NoError(t, extractTarLayer(bytes.NewReader(second), destDir))

	_, err := os.Stat(filepath.Join(destDir, "etc/hosts"))
	assert.True(t, errors.Is(err, os.ErrNotExist))
	_, err = os.Stat(filepath.Join(destDir, "etc/.wh.hosts"))
	assert.True(t, errors.Is(err, os.ErrNotExist))
}

func TestExtractOpaqueWhiteoutClearsDirectory(t *testing.T) {
	destDir := t.TempDir()

	first := writeTar(t, []tarEntry{
		{header: tar.Header{Name: "opt/", Typeflag: tar.TypeDir, Mode: 0o755}},
		{header: tar.Header{Name: "opt/a", Typeflag: tar.TypeReg, Mode: 0o644}, body: []byte("a")},
		{header: tar.Header{Name: "opt/b", Typeflag: tar.TypeReg, Mode: 0o644}, body: []byte("b")},
	})
	require.NoError(t, extractTarLayer(bytes.NewReader(first), destDir))

	second := writeTar(t, []tarEntry{
		{header: tar.Header{Name: "opt/.wh..wh..opq", Typeflag: tar.TypeReg, Mode: 0o644}},
		{header: tar.Header{Name: "opt/c", Typeflag: tar.TypeReg, Mode: 0o644}, body: []byte("c")},
	})
	require.NoError(t, extractTarLayer(bytes.NewReader(second), destDir))

	for _, gone := range []string{"opt/a", "opt/b", "opt/.wh..wh..opq"} {
		_, err := os.Stat(filepath.Join(destDir, gone))
		assert.True(t, errors.Is(err, os.ErrNotExist), "expected %s removed", gone)
	}
	content, err := os.ReadFile(filepath.Join(destDir, "opt/c"))
	require.NoError(t, err)
	assert.Equal(t, "c", string(content))
}

// Absolute paths and ".." traversal are normalized by safeJoinRel so the
// extracted file lands inside destDir.
func TestExtractRejectsAbsoluteAndTraversalPaths(t *testing.T) {
	cases := []struct {
		name      string
		entryName string
	}{
		{"AbsolutePath", "/etc/passwd"},
		{"ParentTraversal", "../../etc/passwd"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			destDir := t.TempDir()
			data := writeTar(t, []tarEntry{
				{header: tar.Header{Name: tc.entryName, Typeflag: tar.TypeReg, Mode: 0o644}, body: []byte("x")},
			})
			require.NoError(t, extractTarLayer(bytes.NewReader(data), destDir))

			out, err := os.ReadFile(filepath.Join(destDir, "etc/passwd"))
			require.NoError(t, err)
			assert.Equal(t, "x", string(out))
		})
	}
}

// Symlink is stored verbatim, but a later write that would traverse it
// outside destDir must be rejected by os.Root.
func TestExtractSymlinkChainEscapeBlocked(t *testing.T) {
	cases := []struct {
		name     string
		linkname string
	}{
		{"AbsoluteEscape", "/etc"},
		{"RelativeEscape", "../../../etc"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			destDir := t.TempDir()
			canary := filepath.Join(destDir, "..", "canary-"+tc.name)
			defer os.Remove(canary)

			data := writeTar(t, []tarEntry{
				{header: tar.Header{Name: "evil", Typeflag: tar.TypeSymlink, Linkname: tc.linkname}},
				{header: tar.Header{Name: "evil/passwd", Typeflag: tar.TypeReg, Mode: 0o644}, body: []byte("pwn")},
			})
			err := extractTarLayer(bytes.NewReader(data), destDir)
			require.Error(t, err, "extraction must fail when escape is attempted")
			assert.ErrorIs(t, err, ErrExtractFailed)

			_, statErr := os.Stat(canary)
			assert.True(t, errors.Is(statErr, os.ErrNotExist),
				"unexpected file outside destDir at %s", canary)
		})
	}
}
