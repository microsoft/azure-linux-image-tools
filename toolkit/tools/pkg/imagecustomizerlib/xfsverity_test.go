// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/configuration"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func verityFileSystemTestStorage(t *testing.T, fsType, mountPath string, verity, inline bool) imagecustomizerapi.Storage {
	t.Helper()
	var storage imagecustomizerapi.Storage
	err := imagecustomizerapi.UnmarshalYaml([]byte(`
bootType: efi
disks:
- partitionTableType: gpt
  partitions:
  - id: esp
    type: esp
    size: 64M
  - id: data
    size: 512M
  - id: hash
    size: 16M
verity:
- id: protected
  name: root
  dataDeviceId: data
  hashDeviceId: hash
filesystems:
- deviceId: esp
  type: fat32
  mountPoint: /boot/efi
- deviceId: protected
  type: xfs
  mountPoint:
    path: /
    options: ro
`), &storage)
	require.NoError(t, err)
	storage.FileSystems[1].Type = imagecustomizerapi.FileSystemType(fsType)
	storage.FileSystems[1].MountPoint.Path = mountPath
	if mountPath == "/usr" {
		storage.Verity[0].Name = "usr"
	}
	if inline {
		storage.Verity[0].HashDeviceId = "data"
	}
	if !verity {
		storage.Verity = nil
		storage.FileSystems[1].DeviceId = "data"
	}
	require.NoError(t, storage.IsValid())
	return storage
}

func TestPartitionToImagerVerityData(t *testing.T) {
	for _, mountPath := range []string{"/", "/usr"} {
		for _, verity := range []bool{false, true} {
			for _, inline := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/verity=%v/inline=%v", mountPath, verity, inline), func(t *testing.T) {
					storage := verityFileSystemTestStorage(t, "xfs", mountPath, verity, inline)
					disk, err := diskConfigToImager(storage.Disks[0], storage.FileSystems)
					require.NoError(t, err)
					assert.Equal(t, verity, disk.Partitions[1].IsVerityData)
					assert.False(t, disk.Partitions[0].IsVerityData)
					assert.False(t, disk.Partitions[2].IsVerityData)
				})
			}
		}
	}
}

func TestFormatVerityFileSystems(t *testing.T) {
	if testing.Short() {
		t.Skip("Short mode enabled")
	}
	if os.Geteuid() != 0 {
		t.Skip("Test must be run as root")
	}

	for _, test := range []struct {
		name      string
		fsType    string
		mountPath string
		verity    bool
		inline    bool
	}{
		{"root-xfs", "xfs", "/", true, false},
		{"usr-xfs", "xfs", "/usr", true, false},
		{"inline-xfs", "xfs", "/", true, true},
		{"plain-root-xfs", "xfs", "/", false, false},
		{"root-ext4", "ext4", "/", true, false},
		{"usr-btrfs", "btrfs", "/usr", true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			storage := verityFileSystemTestStorage(t, test.fsType, test.mountPath, test.verity, test.inline)
			disk, err := diskConfigToImager(storage.Disks[0], storage.FileSystems)
			require.NoError(t, err)
			dir := t.TempDir()
			diskPath := filepath.Join(dir, "disk.raw")
			require.NoError(t, diskutils.CreateSparseDisk(diskPath, disk.MaxSize, 0o600))
			loop, err := safeloopback.NewLoopback(diskPath)
			require.NoError(t, err)
			t.Cleanup(func() { assert.NoError(t, loop.CleanClose()) })
			devices, _, _, err := diskutils.CreatePartitions(targetos.TargetOsAzureLinux3, loop.DevicePath(),
				disk, configuration.RootEncryption{})
			require.NoError(t, err)
			dataPath := devices["data"]
			logicalSector, _, err := shell.Execute("blockdev", "--getss", dataPath)
			require.NoError(t, err)
			require.Equal(t, "512", strings.TrimSpace(logicalSector))

			if test.fsType == "xfs" {
				data, err := os.Open(dataPath)
				require.NoError(t, err)
				sector := make([]byte, 2)
				_, err = data.ReadAt(sector, 102) // XFS sb_sectsize.
				require.NoError(t, data.Close())
				require.NoError(t, err)
				expected := uint16(512)
				if test.verity {
					expected = 4096
				}
				assert.Equal(t, expected, binary.BigEndian.Uint16(sector))
			}

			mountPath := filepath.Join(dir, "mount")
			mount, err := safemount.NewMount(dataPath, mountPath, test.fsType, 0, "", true)
			require.NoError(t, err)
			t.Cleanup(mount.Close)
			require.NoError(t, os.WriteFile(filepath.Join(mountPath, "marker"), []byte("verity-data\n"), 0o644))
			require.NoError(t, mount.CleanClose())
			if !test.verity {
				return
			}

			hashPath := devices["hash"]
			offsetArgs := []string(nil)
			formatArgs := []string{"format", "--data-block-size=4096", "--hash-block-size=4096"}
			if test.inline {
				hashPath = dataPath
				dataSize := storage.FileSystems[1].Size
				offsetArgs = []string{"--hash-offset", fmt.Sprint(dataSize)}
				formatArgs = append(formatArgs, "--data-blocks", fmt.Sprint(dataSize/4096))
			}
			formatArgs = append(formatArgs, offsetArgs...)
			output, _, err := shell.Execute("veritysetup", append(formatArgs, dataPath, hashPath)...)
			require.NoError(t, err)
			match := regexp.MustCompile(`Root hash:\s+([0-9a-f]+)`).FindStringSubmatch(output)
			require.Len(t, match, 2)
			_, _, err = shell.Execute("veritysetup", append([]string{"verify", dataPath, hashPath, match[1]}, offsetArgs...)...)
			require.NoError(t, err)
			name := fmt.Sprintf("ic-xfs-%d-%s", os.Getpid(), filepath.Base(filepath.Dir(dir)))
			_, _, err = shell.Execute("veritysetup", append([]string{"open", dataPath, name, hashPath, match[1]}, offsetArgs...)...)
			require.NoError(t, err)
			t.Cleanup(func() {
				_, _, err := shell.Execute("veritysetup", "close", name)
				assert.NoError(t, err)
			})
			mappedPath := filepath.Join("/dev/mapper", name)
			logicalSector, _, err = shell.Execute("blockdev", "--getss", mappedPath)
			require.NoError(t, err)
			require.Equal(t, "4096", strings.TrimSpace(logicalSector))
			mappedMount, err := safemount.NewMount(mappedPath, mountPath, test.fsType, unix.MS_RDONLY, "", true)
			require.NoError(t, err)
			t.Cleanup(mappedMount.Close)
			marker, err := os.ReadFile(filepath.Join(mountPath, "marker"))
			require.NoError(t, err)
			assert.Equal(t, "verity-data\n", string(marker))
			require.NoError(t, mappedMount.CleanClose())
		})
	}
}
