// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/moby/sys/mountinfo"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

// makeAclEtcOverlayTestRoot creates a fake ACL image root:
//   - a baseline (usr/share/distro/etc) with passwd, chrony.conf, obsolete.conf, and a subdirectory
//   - a physical /etc with a shadowing chrony.conf, a stale.conf, and a machine-id
func makeAclEtcOverlayTestRoot(t *testing.T, testName string) string {
	rootDir := filepath.Join(tmpDir, testName)

	baselineDir := filepath.Join(rootDir, aclEtcBaselineDir)
	err := os.MkdirAll(filepath.Join(baselineDir, "baseline-dir"), os.ModePerm)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(baselineDir, "passwd"), []byte("root:x:0:0:root:/root:/bin/bash\n"), 0o644)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(baselineDir, "chrony.conf"), []byte("baseline\n"), 0o644)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(baselineDir, "obsolete.conf"), []byte("obsolete\n"), 0o644)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(baselineDir, "baseline-dir", "keep.conf"), []byte("keep\n"), 0o644)
	assert.NoError(t, err)

	etcDir := filepath.Join(rootDir, "etc")
	err = os.MkdirAll(etcDir, os.ModePerm)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(etcDir, "chrony.conf"), []byte("physical\n"), 0o644)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(etcDir, "stale.conf"), []byte("stale\n"), 0o644)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(etcDir, "machine-id"), []byte("0123456789abcdef\n"), 0o444)
	assert.NoError(t, err)

	// Distinctive mode, so the tests can verify /etc's own attributes are carried through.
	err = os.Chmod(etcDir, 0o751)
	assert.NoError(t, err)

	return rootDir
}

func assertFileContent(t *testing.T, path string, expected string) {
	content, err := os.ReadFile(path)
	if assert.NoError(t, err, "read (%s)", path) {
		assert.Equal(t, expected, string(content), "content of (%s)", path)
	}
}

func TestAclEtcOverlayFold(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test must be run as root because it mounts an overlay filesystem")
	}

	rootDir := makeAclEtcOverlayTestRoot(t, "TestAclEtcOverlayFold")
	defer os.RemoveAll(rootDir)

	etcDir := filepath.Join(rootDir, "etc")
	baselineDir := filepath.Join(rootDir, aclEtcBaselineDir)

	overlay, err := newAclEtcOverlay(context.Background(), rootDir, imagecustomizerapi.ReinitializeVerityTypeAll)
	if !assert.NoError(t, err) {
		return
	}
	defer overlay.Close()

	// The merged view composes both layers: the baseline shows through, and the physical /etc
	// shadows it where both define a file.
	assertFileContent(t, filepath.Join(etcDir, "passwd"), "root:x:0:0:root:/root:/bin/bash\n")
	assertFileContent(t, filepath.Join(etcDir, "chrony.conf"), "physical\n")
	assertFileContent(t, filepath.Join(etcDir, "baseline-dir", "keep.conf"), "keep\n")

	// The merged /etc presents the physical /etc's own directory attributes.
	etcInfo, err := os.Stat(etcDir)
	assert.NoError(t, err)
	assert.Equal(t, os.FileMode(0o751), etcInfo.Mode().Perm(), "merged /etc must keep the physical /etc's mode")

	// Simulate customization: add a file, modify a baseline file, delete an upper file and a
	// baseline file, and create a directory.
	err = os.WriteFile(filepath.Join(etcDir, "new.conf"), []byte("new\n"), 0o644)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(etcDir, "passwd"),
		[]byte("root:x:0:0:root:/root:/bin/bash\nrpc:x:31:31::/:/bin/false\n"), 0o644)
	assert.NoError(t, err)

	err = os.Remove(filepath.Join(etcDir, "stale.conf"))
	assert.NoError(t, err)

	err = os.Remove(filepath.Join(etcDir, "obsolete.conf"))
	assert.NoError(t, err)

	err = os.MkdirAll(filepath.Join(etcDir, "new-dir"), os.ModePerm)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(etcDir, "new-dir", "sub.conf"), []byte("sub\n"), 0o644)
	assert.NoError(t, err)

	err = overlay.Finalize(context.Background())
	assert.NoError(t, err)

	mounted, err := mountinfo.Mounted(etcDir)
	assert.NoError(t, err)
	assert.False(t, mounted, "/etc must be unmounted after finalize")

	// The baseline is now the merged /etc.
	assertFileContent(t, filepath.Join(baselineDir, "passwd"),
		"root:x:0:0:root:/root:/bin/bash\nrpc:x:31:31::/:/bin/false\n")
	assertFileContent(t, filepath.Join(baselineDir, "new.conf"), "new\n")
	assertFileContent(t, filepath.Join(baselineDir, "chrony.conf"), "physical\n")
	assertFileContent(t, filepath.Join(baselineDir, "new-dir", "sub.conf"), "sub\n")
	assertFileContent(t, filepath.Join(baselineDir, "baseline-dir", "keep.conf"), "keep\n")

	assert.NoFileExists(t, filepath.Join(baselineDir, "stale.conf"), "deleted upper file must not be folded")
	assert.NoFileExists(t, filepath.Join(baselineDir, "obsolete.conf"), "whiteout must be honored")
	assert.NoFileExists(t, filepath.Join(baselineDir, "machine-id"), "machine-id must never be folded")

	// The new baseline directory carries /etc's own attributes.
	baselineInfo, err := os.Stat(baselineDir)
	assert.NoError(t, err)
	assert.Equal(t, os.FileMode(0o751), baselineInfo.Mode().Perm(), "baseline must keep /etc's mode")

	// The physical /etc is empty and all scratch state is gone.
	entries, err := os.ReadDir(etcDir)
	assert.NoError(t, err)
	assert.Empty(t, entries, "physical /etc must be empty after fold")

	assert.NoDirExists(t, filepath.Join(rootDir, aclEtcOverlayWorkDirName))
	assert.NoDirExists(t, filepath.Join(filepath.Dir(baselineDir), aclEtcOverlayFoldTmpName))
}

func TestAclEtcOverlayReadOnlyBaseline(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test must be run as root because it mounts filesystems")
	}

	rootDir := makeAclEtcOverlayTestRoot(t, "TestAclEtcOverlayReadOnlyBaseline")
	defer os.RemoveAll(rootDir)

	etcDir := filepath.Join(rootDir, "etc")
	baselineDir := filepath.Join(rootDir, aclEtcBaselineDir)

	// Make the baseline read-only, like a verity-protected /usr that is not being reinitialized.
	err := unix.Mount(baselineDir, baselineDir, "", unix.MS_BIND, "")
	if !assert.NoError(t, err) {
		return
	}
	defer unix.Unmount(baselineDir, 0)
	err = unix.Mount("", baselineDir, "", unix.MS_BIND|unix.MS_REMOUNT|unix.MS_RDONLY, "")
	if !assert.NoError(t, err) {
		return
	}

	overlay, err := newAclEtcOverlay(context.Background(), rootDir, imagecustomizerapi.ReinitializeVerityTypeNone)
	if !assert.NoError(t, err) {
		return
	}
	defer overlay.Close()

	// Customize through the merged view: add a file and modify a baseline file.
	err = os.WriteFile(filepath.Join(etcDir, "new.conf"), []byte("new\n"), 0o644)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(etcDir, "passwd"),
		[]byte("root:x:0:0:root:/root:/bin/bash\nrpc:x:31:31::/:/bin/false\n"), 0o644)
	assert.NoError(t, err)

	err = overlay.Finalize(context.Background())
	assert.NoError(t, err)

	mounted, err := mountinfo.Mounted(etcDir)
	assert.NoError(t, err)
	assert.False(t, mounted, "/etc must be unmounted after finalize")

	// The baseline is untouched; the changes stay on the root partition as the upper layer, just
	// like changes made on a booted system.
	assertFileContent(t, filepath.Join(baselineDir, "passwd"), "root:x:0:0:root:/root:/bin/bash\n")
	assert.NoFileExists(t, filepath.Join(baselineDir, "new.conf"))

	assertFileContent(t, filepath.Join(etcDir, "new.conf"), "new\n")
	assertFileContent(t, filepath.Join(etcDir, "passwd"),
		"root:x:0:0:root:/root:/bin/bash\nrpc:x:31:31::/:/bin/false\n")
	assertFileContent(t, filepath.Join(etcDir, "stale.conf"), "stale\n")
	assertFileContent(t, filepath.Join(etcDir, "machine-id"), "0123456789abcdef\n")

	assert.NoDirExists(t, filepath.Join(rootDir, aclEtcOverlayWorkDirName))
}

func TestAclEtcOverlayCloseWithoutFinalize(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test must be run as root because it mounts an overlay filesystem")
	}

	rootDir := makeAclEtcOverlayTestRoot(t, "TestAclEtcOverlayCloseWithoutFinalize")
	defer os.RemoveAll(rootDir)

	etcDir := filepath.Join(rootDir, "etc")
	baselineDir := filepath.Join(rootDir, aclEtcBaselineDir)

	overlay, err := newAclEtcOverlay(context.Background(), rootDir, imagecustomizerapi.ReinitializeVerityTypeAll)
	if !assert.NoError(t, err) {
		return
	}

	// Write through the overlay, then abandon.
	err = os.WriteFile(filepath.Join(etcDir, "new.conf"), []byte("new\n"), 0o644)
	assert.NoError(t, err)

	overlay.Close()

	mounted, err := mountinfo.Mounted(etcDir)
	assert.NoError(t, err)
	assert.False(t, mounted, "/etc must be unmounted after Close")

	// The baseline is untouched and the scratch state is gone. The abandoned write remains in the
	// physical /etc (the upper layer); the image is expected to be discarded.
	assertFileContent(t, filepath.Join(baselineDir, "chrony.conf"), "baseline\n")
	assert.NoFileExists(t, filepath.Join(baselineDir, "new.conf"))
	assertFileContent(t, filepath.Join(etcDir, "new.conf"), "new\n")
	assertFileContent(t, filepath.Join(etcDir, "stale.conf"), "stale\n")
	assertFileContent(t, filepath.Join(etcDir, "machine-id"), "0123456789abcdef\n")

	// Close deliberately leaves the scratch state behind: a failed customization's image is never
	// output.
	assert.DirExists(t, filepath.Join(rootDir, aclEtcOverlayWorkDirName))
}

func TestAclEtcOverlayMissingBaseline(t *testing.T) {
	rootDir := filepath.Join(tmpDir, "TestAclEtcOverlayMissingBaseline")
	defer os.RemoveAll(rootDir)

	err := os.MkdirAll(filepath.Join(rootDir, "etc"), os.ModePerm)
	assert.NoError(t, err)

	_, err = newAclEtcOverlay(context.Background(), rootDir, imagecustomizerapi.ReinitializeVerityTypeAll)
	assert.ErrorIs(t, err, ErrAclEtcOverlayAssemble)
	assert.ErrorContains(t, err, "baseline")
}
