// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package safechroot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"

	"github.com/stretchr/testify/assert"
)

const (
	testTar    = "testchroot.tar.gz"
	emptyPath  = ""
	emptyFlags = 0
)

var (
	testDir string
)

func TestMain(m *testing.M) {
	logger.InitStderrLog()

	var retVal int
	if os.Geteuid() != 0 {
		// We're not running as root; we need to skip all tests in this file.
		logger.Log.Warn("safechroot tests must be run as root; skipping...")
		retVal = 0
	} else {
		// We're running as root; let's proceed with test setup and testing.
		var err error
		testDir, err = filepath.Abs("testdata")
		if err != nil {
			logger.Log.Panicf("Failed to get path to test data, error: %s", err)
		}

		retVal = m.Run()
	}

	os.Exit(retVal)
}

func TestInitializeShouldCreateRoot(t *testing.T) {
	extraMountPoints := []*MountPoint{}
	extraDirectories := []string{}

	dir := filepath.Join(t.TempDir(), "TestInitializeShouldCreateRoot")
	chroot := NewChroot(dir, false)

	err := chroot.Initialize(emptyPath, extraDirectories, extraMountPoints, true)
	assert.NoError(t, err)

	defer chroot.Close()

	_, err = os.Stat(chroot.RootDir())
	assert.True(t, !os.IsNotExist(err))
}

func TestCloseShouldRemoveRoot(t *testing.T) {
	extraMountPoints := []*MountPoint{}
	extraDirectories := []string{}

	dir := filepath.Join(t.TempDir(), "TestCloseShouldRemoveRoot")
	chroot := NewChroot(dir, false)

	err := chroot.Initialize(emptyPath, extraDirectories, extraMountPoints, true)
	assert.NoError(t, err)

	// save away chroot location and close
	chrootDir := chroot.RootDir()
	err = chroot.Close()
	assert.NoError(t, err)

	_, err = os.Stat(chrootDir)
	assert.True(t, os.IsNotExist(err))
}

func TestRootDirShouldReturnRootDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "TestRootDirShouldReturnRootDir")
	chroot := NewChroot(dir, false)
	assert.Equal(t, dir, chroot.RootDir())
}

func TestInitializeShouldExtractTar(t *testing.T) {
	const expectedFile = "/test/testfile.txt"

	tarPath := filepath.Join(testDir, testTar)
	extraMountPoints := []*MountPoint{}
	extraDirectories := []string{}

	dir := filepath.Join(t.TempDir(), "TestInitializeShouldExtractTar")
	chroot := NewChroot(dir, false)

	err := chroot.Initialize(tarPath, extraDirectories, extraMountPoints, true)
	assert.NoError(t, err)
	defer chroot.Close()

	fullPath := filepath.Join(chroot.RootDir(), expectedFile)
	_, err = os.Stat(fullPath)
	assert.True(t, !os.IsNotExist(err))
}

func TestInitializeShouldCreateCustomMountPoints(t *testing.T) {
	const expectedFile = "/custom-mount/testfile.txt"

	extraDirectories := []string{}
	srcMount := filepath.Join(testDir, "testmount")
	extraMountPoints := []*MountPoint{
		NewMountPoint(srcMount, "custom-mount", "", BindMountPointFlags, emptyPath),
	}

	dir := filepath.Join(t.TempDir(), "TestInitializeShouldCreateCustomMountPoints")
	chroot := NewChroot(dir, false)

	err := chroot.Initialize(emptyPath, extraDirectories, extraMountPoints, true)
	assert.NoError(t, err)
	defer chroot.Close()

	fullPath := filepath.Join(dir, expectedFile)
	_, err = os.Stat(fullPath)
	assert.True(t, !os.IsNotExist(err))
}

func TestInitializeShouldCleanupOnBadMountPoint(t *testing.T) {
	const invalidMountPointSource = "@"

	extraDirectories := []string{}
	extraMountPoints := []*MountPoint{
		NewMountPoint(invalidMountPointSource, "custom-mount", "", emptyFlags, emptyPath),
	}

	dir := filepath.Join(t.TempDir(), "TestInitializeShouldCleanupOnBadMountPoint")
	chroot := NewChroot(dir, false)

	err := chroot.Initialize(emptyPath, extraDirectories, extraMountPoints, true)
	assert.Error(t, err)

	_, err = os.Stat(dir)
	assert.True(t, os.IsNotExist(err))
}

func TestInitializeShouldCreateExtraDirectories(t *testing.T) {
	const expectedExtraDirectory = "/testdir"

	extraDirectories := []string{expectedExtraDirectory}
	extraMountPoints := []*MountPoint{}

	dir := filepath.Join(t.TempDir(), "TestInitializeShouldCreateExtraDirectories")
	chroot := NewChroot(dir, false)

	err := chroot.Initialize(emptyPath, extraDirectories, extraMountPoints, true)
	assert.NoError(t, err)
	defer chroot.Close()

	fullPath := filepath.Join(chroot.RootDir(), expectedExtraDirectory)
	_, err = os.Stat(fullPath)
	assert.True(t, !os.IsNotExist(err))
}

// containsFileNamed reports whether a file with the given name exists anywhere under
// root. filepath.WalkDir does not follow symlinks, so it will not traverse an escape
// link out of root.
func containsFileNamed(root, name string) bool {
	found := false
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == name {
			found = true
		}
		return nil
	})
	return found
}

// containsFileUnderParent reports whether a file 'name' whose immediate parent directory
// is 'parent' exists anywhere under root. WalkDir does not follow symlinks, so a match
// proves the file landed within root rather than through an escape link.
func containsFileUnderParent(root, parent, name string) bool {
	found := false
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == name && filepath.Base(filepath.Dir(p)) == parent {
			found = true
		}
		return nil
	})
	return found
}

func TestAddFilesToDestinationClampsSymlinkEscape(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	assert.NoError(t, os.MkdirAll(root, 0o755))

	// A directory OUTSIDE the root that a mistaken/malicious symlink points at.
	hostArea := filepath.Join(t.TempDir(), "host")
	assert.NoError(t, os.MkdirAll(hostArea, 0o755))

	// Inside the root, an absolute symlink 'escape' -> hostArea, like a preserved
	// 'etc -> /' link. Without SecureJoin, writing through it lands in hostArea.
	assert.NoError(t, os.Symlink(hostArea, filepath.Join(root, "escape")))

	srcFile := filepath.Join(t.TempDir(), "src.txt")
	assert.NoError(t, os.WriteFile(srcFile, []byte("payload"), 0o644))

	// copyFile path (FileToCopy.Src).
	err := AddFilesToDestination(root, FileToCopy{Src: srcFile, Dest: "/escape/copied.txt"})
	assert.NoError(t, err)

	// writeFile path (FileToCopy.Content).
	content := "written"
	err = AddFilesToDestination(root, FileToCopy{Content: &content, Dest: "/escape/written.txt"})
	assert.NoError(t, err)

	// The escape is clamped: nothing was written into the real hostArea.
	entries, err := os.ReadDir(hostArea)
	assert.NoError(t, err)
	assert.Empty(t, entries, "files must not escape the chroot root into %s", hostArea)

	// The writes still succeeded, resolved within root.
	assert.True(t, containsFileNamed(root, "copied.txt"))
	assert.True(t, containsFileNamed(root, "written.txt"))
}

func TestAddDirsClampsSymlinkEscape(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	assert.NoError(t, os.MkdirAll(root, 0o755))

	hostArea := filepath.Join(t.TempDir(), "host")
	assert.NoError(t, os.MkdirAll(hostArea, 0o755))

	// 'escape' -> hostArea inside root.
	assert.NoError(t, os.Symlink(hostArea, filepath.Join(root, "escape")))

	// Source directory with a file to copy in.
	srcDir := filepath.Join(t.TempDir(), "srcdir")
	assert.NoError(t, os.MkdirAll(srcDir, 0o755))
	assert.NoError(t, os.WriteFile(filepath.Join(srcDir, "inner.txt"), []byte("payload"), 0o644))

	chroot := NewChroot(root, true)
	err := chroot.AddDirs(DirToCopy{Src: srcDir, Dest: "/escape/copied", NewDirPermissions: 0o755, ChildFilePermissions: 0o644})
	assert.NoError(t, err)

	entries, err := os.ReadDir(hostArea)
	assert.NoError(t, err)
	assert.Empty(t, entries, "directory copy must not escape the chroot root into %s", hostArea)

	// The copy still lands within root, preserving the 'copied/inner.txt' structure
	// ('copied' is a real directory created after the clamped 'escape' link).
	assert.True(t, containsFileUnderParent(root, "copied", "inner.txt"),
		"expected copied/inner.txt to be created within root")
}
