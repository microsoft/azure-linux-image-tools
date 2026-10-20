// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestErofsEditsPreserveReadonlyMounts(t *testing.T) {
	for _, fsType := range []string{"ext4", "erofs", "btrfs"} {
		t.Run(fsType, func(t *testing.T) {
			mount := safechroot.NewMountPoint("/not-a-device", "/usr", fsType, unix.MS_RDONLY, "")
			mounts := []*safechroot.MountPoint{mount}
			edits := &erofsFilesystemEdits{}
			err := edits.prepare(imageconnection.NewImageConnection(), mounts,
				[]diskutils.PartitionInfo{{Type: "part", Path: "/not-a-device", FileSystemType: fsType}},
				t.TempDir(), nil)
			require.NoError(t, err)
			require.Same(t, mount, mounts[0])
			require.Empty(t, edits.stages)
		})
	}
}

func TestErofsEditsIgnoreOtherFilesystems(t *testing.T) {
	for _, fsType := range []string{"ext4", "btrfs"} {
		t.Run(fsType, func(t *testing.T) {
			mount := safechroot.NewMountPoint("/not-a-device", "/usr", fsType, 0, "")
			mounts := []*safechroot.MountPoint{mount}
			edits := &erofsFilesystemEdits{}
			require.NoError(t, edits.prepare(imageconnection.NewImageConnection(), mounts,
				[]diskutils.PartitionInfo{{Type: "part", Path: "/not-a-device", FileSystemType: fsType}},
				t.TempDir(), nil))
			require.Same(t, mount, mounts[0])
			require.Empty(t, edits.stages)
		})
	}
}

func TestErofsEditsRejectUnsupportedLayouts(t *testing.T) {
	for _, tc := range []struct {
		name     string
		target   string
		settings verityFormatSettings
		wantErr  string
	}{
		{name: "root", target: "/", wantErr: "EROFS root filesystem"},
		{name: "inline", target: "/usr", settings: verityFormatSettings{hashOffsetBytes: 4096},
			wantErr: "separate, whole-partition verity"},
		{name: "partial", target: "/usr", settings: verityFormatSettings{dataSizeBytes: 1024},
			wantErr: "separate, whole-partition verity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			edits := &erofsFilesystemEdits{}
			err := edits.prepare(imageconnection.NewImageConnection(),
				[]*safechroot.MountPoint{safechroot.NewMountPoint("/not-a-device", tc.target, "erofs", 0, "")},
				[]diskutils.PartitionInfo{{Type: "part", Path: "/not-a-device", PartUuid: "usr",
					FileSystemType: "erofs", SizeInBytes: 4096}},
				t.TempDir(), []verityDeviceMetadata{{dataPartUuid: "usr", formatSettings: tc.settings}})
			require.ErrorContains(t, err, tc.wantErr)
			require.Empty(t, edits.stages)
		})
	}
}

func TestErofsOverflowDoesNotWriteDestination(t *testing.T) {
	dir := t.TempDir()
	image := filepath.Join(dir, "image")
	destination := filepath.Join(dir, "destination")
	require.NoError(t, os.WriteFile(image, make([]byte, 4096), 0o600))
	require.NoError(t, os.WriteFile(destination, []byte("unchanged"), 0o600))
	err := writeErofsPartition(image, diskutils.PartitionInfo{Path: destination, SizeInBytes: 512})
	require.ErrorContains(t, err, "exceeds partition capacity")
	data, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, "unchanged", string(data))
}

func TestErofsRejectsNonBlockDestination(t *testing.T) {
	dir := t.TempDir()
	image := filepath.Join(dir, "image")
	destination := filepath.Join(dir, "destination")
	require.NoError(t, os.WriteFile(image, make([]byte, 512), 0o600))
	require.NoError(t, os.WriteFile(destination, []byte("unchanged"), 0o600))
	err := writeErofsPartition(image, diskutils.PartitionInfo{Path: destination, SizeInBytes: 4096})
	require.ErrorContains(t, err, "not a block device")
	data, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, "unchanged", string(data))
}

func TestFindRootfsPartitionErofs(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root, loop devices and EROFS support")
	}
	for _, tc := range []struct {
		name     string
		markers  []string
		rootPath string
		wantErr  string
	}{
		{name: "root", markers: []string{"etc/fstab"}},
		{name: "acl-usr", markers: []string{"share/ic/etc/fstab"}, rootPath: "share/ic"},
		{name: "missing", wantErr: "failed to find rootfs partition"},
		{name: "ambiguous", markers: []string{"etc/fstab", "share/ic/etc/fstab"},
			wantErr: "found too many rootfs partition candidates"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "tree")
			require.NoError(t, os.Mkdir(source, 0o755))
			for _, marker := range tc.markers {
				path := filepath.Join(source, marker)
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, []byte("/dev/mapper/usr /usr erofs ro 0 0\n"), 0o644))
			}
			image := filepath.Join(dir, "usr.erofs")
			require.NoError(t, shell.ExecuteLive(true, "mkfs.erofs", image, source))
			connection := imageconnection.NewImageConnection()
			defer connection.Close()
			require.NoError(t, connection.ConnectLoopback(image))
			partition := diskutils.PartitionInfo{Type: "part", Path: connection.Loopback().DevicePath(),
				FileSystemType: "erofs"}
			found, rootPath, err := findRootfsPartition([]diskutils.PartitionInfo{partition}, dir)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				require.Nil(t, found)
			} else {
				require.NoError(t, err)
				require.Equal(t, &partition, found)
				require.Equal(t, tc.rootPath, rootPath)
			}
			require.NoError(t, connection.CleanClose())
		})
	}
}

func TestErofsCustomizationRoundtrip(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root, loop devices and EROFS support")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "tree")
	require.NoError(t, os.Mkdir(source, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(source, "original"), []byte("before"), 0o751))
	require.NoError(t, os.Link(filepath.Join(source, "original"), filepath.Join(source, "hardlink")))
	require.NoError(t, os.Symlink("original", filepath.Join(source, "symlink")))
	require.NoError(t, unix.Setxattr(filepath.Join(source, "original"), "user.proof", []byte("preserved"), 0))
	image := filepath.Join(dir, "usr.erofs")
	fsUuid := "f3b4e028-bd04-491c-bd18-2ad98b9f81cf"
	require.NoError(t, shell.ExecuteLive(true, "mkfs.erofs", "-U", fsUuid, "-L", "USR-A", image, source))
	f, err := os.OpenFile(image, os.O_WRONLY, 0)
	require.NoError(t, err)
	require.NoError(t, f.Truncate(64*1024*1024))
	require.NoError(t, f.Close())

	connection := imageconnection.NewImageConnection()
	defer connection.Close()
	require.NoError(t, connection.ConnectLoopback(image))
	partition := diskutils.PartitionInfo{Type: "part", Path: connection.Loopback().DevicePath(),
		FileSystemType: "erofs", Uuid: fsUuid, Label: "USR-A", SizeInBytes: 64 * 1024 * 1024}
	mounts := []*safechroot.MountPoint{safechroot.NewMountPoint(partition.Path, "/usr", "erofs", 0, "")}
	edits := &erofsFilesystemEdits{}
	require.NoError(t, edits.prepare(connection, mounts, []diskutils.PartitionInfo{partition}, dir, nil))
	require.Equal(t, "none", mounts[0].GetFSType())
	require.NotZero(t, mounts[0].GetFlags()&unix.MS_BIND)
	require.NoError(t, os.WriteFile(filepath.Join(edits.stages[0].directory, "original"), []byte("after"), 0o751))
	require.NoError(t, edits.finalize())

	output := filepath.Join(dir, "mounted")
	mount, err := safemount.NewMount(partition.Path, output, "erofs", unix.MS_RDONLY, "", true)
	require.NoError(t, err)
	defer mount.Close()
	data, err := os.ReadFile(filepath.Join(output, "hardlink"))
	require.NoError(t, err)
	require.Equal(t, "after", string(data))
	target, err := os.Readlink(filepath.Join(output, "symlink"))
	require.NoError(t, err)
	require.Equal(t, "original", target)
	attributes := make([]byte, 64)
	n, err := unix.Getxattr(filepath.Join(output, "original"), "user.proof", attributes)
	require.NoError(t, err)
	require.Equal(t, "preserved", string(attributes[:n]))
	info, err := os.Stat(filepath.Join(output, "original"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o751), info.Mode().Perm())
	require.NoError(t, mount.CleanClose())
}
