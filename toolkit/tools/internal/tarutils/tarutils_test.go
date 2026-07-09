// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package tarutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	logger.InitStderrLog()
	retVal := m.Run()
	os.Exit(retVal)
}

// TestCreateTarGzArchive_PreservesSymlinks round-trips a directory holding a regular file, a relative symlink
// to that file, and a symlink to a subdirectory, and verifies the symlinks survive as symlinks (not dereferenced) with
// their targets intact.
func TestCreateTarGzArchive_PreservesSymlinks(t *testing.T) {
	sourceDir := t.TempDir()

	regularPath := filepath.Join(sourceDir, "regular.txt")
	err := os.WriteFile(regularPath, []byte("hello"), 0o644)
	assert.NoError(t, err)

	subDir := filepath.Join(sourceDir, "subdir")
	err = os.Mkdir(subDir, 0o755)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(subDir, "inner.txt"), []byte("inner"), 0o644)
	assert.NoError(t, err)

	// Two relative symlinks, one pointing at the regular file and one at the subdirectory.
	err = os.Symlink("regular.txt", filepath.Join(sourceDir, "file-link"))
	assert.NoError(t, err)
	err = os.Symlink("subdir", filepath.Join(sourceDir, "dir-link"))
	assert.NoError(t, err)

	archivePath := filepath.Join(t.TempDir(), "archive.tar.gz")
	err = CreateTarGzArchive(sourceDir, archivePath)
	assert.NoError(t, err)

	outDir := t.TempDir()
	err = ExpandTarGzArchive(archivePath, outDir)
	assert.NoError(t, err)

	// The regular file's content survives.
	gotRegular, err := os.ReadFile(filepath.Join(outDir, "regular.txt"))
	assert.NoError(t, err)
	assert.Equal(t, "hello", string(gotRegular))

	// The file symlink survives as a symlink pointing at the same target.
	fileLinkInfo, err := os.Lstat(filepath.Join(outDir, "file-link"))
	assert.NoError(t, err)
	assert.NotZero(t, fileLinkInfo.Mode()&os.ModeSymlink, "file-link should be a symlink")
	fileLinkTarget, err := os.Readlink(filepath.Join(outDir, "file-link"))
	assert.NoError(t, err)
	assert.Equal(t, "regular.txt", fileLinkTarget)

	// The directory symlink survives as a symlink rather than a dereferenced copy of the directory.
	dirLinkInfo, err := os.Lstat(filepath.Join(outDir, "dir-link"))
	assert.NoError(t, err)
	assert.NotZero(t, dirLinkInfo.Mode()&os.ModeSymlink, "dir-link should be a symlink")
	dirLinkTarget, err := os.Readlink(filepath.Join(outDir, "dir-link"))
	assert.NoError(t, err)
	assert.Equal(t, "subdir", dirLinkTarget)
}

// TestExpandTarGzArchive_RejectsAbsoluteSymlink verifies extraction refuses a symlink whose target is absolute, since it
// could point outside the expansion root.
func TestExpandTarGzArchive_RejectsAbsoluteSymlink(t *testing.T) {
	sourceDir := t.TempDir()
	err := os.Symlink("/etc/passwd", filepath.Join(sourceDir, "evil-link"))
	assert.NoError(t, err)

	archivePath := filepath.Join(t.TempDir(), "archive.tar.gz")
	err = CreateTarGzArchive(sourceDir, archivePath)
	assert.NoError(t, err)

	err = ExpandTarGzArchive(archivePath, t.TempDir())
	assert.ErrorContains(t, err, "unallowed symlink in archive")
}

// TestExpandTarGzArchive_RejectsParentTraversalSymlink verifies extraction refuses a relative symlink whose target
// escapes the expansion root via "..".
func TestExpandTarGzArchive_RejectsParentTraversalSymlink(t *testing.T) {
	sourceDir := t.TempDir()
	err := os.Symlink("../escape", filepath.Join(sourceDir, "evil-link"))
	assert.NoError(t, err)

	archivePath := filepath.Join(t.TempDir(), "archive.tar.gz")
	err = CreateTarGzArchive(sourceDir, archivePath)
	assert.NoError(t, err)

	err = ExpandTarGzArchive(archivePath, t.TempDir())
	assert.ErrorContains(t, err, "unallowed symlink in archive")
}
