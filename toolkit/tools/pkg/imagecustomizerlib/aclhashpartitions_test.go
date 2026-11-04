// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/cosiapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerityCosiHashPartitionCapacity(t *testing.T) {
	if testing.Short() {
		t.Skip("Short mode enabled")
	}
	if os.Geteuid() != 0 {
		t.Skip("Test must be run as root")
	}

	for _, test := range []struct {
		name     string
		acl      bool
		hashSize uint64
	}{
		{name: "acl", acl: true, hashSize: 17 * diskutils.MiB},
		{name: "generic", hashSize: 16916480},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			basePath, _ := buildSyntheticAclDiskWithHashPartitions(t, dir)
			diskPath := filepath.Join(dir, "grown.raw")
			acl := &imagecustomizerapi.Acl{
				Usr: &imagecustomizerapi.AclPartitionGrow{Size: imagecustomizerapi.DiskSize(2 * diskutils.GiB)},
			}
			require.NoError(t, growAclStandardPartitions(context.Background(), acl, basePath, diskPath))
			connection := &osImageConnection{imageConnection: imageconnection.NewImageConnection()}
			require.NoError(t, connection.imageConnection.ConnectLoopback(diskPath))
			defer connection.Close()
			dev := connection.Loopback().DevicePath()
			parts, err := diskutils.GetDiskPartitions(dev)
			require.NoError(t, err)
			byLabel := partitionsByLabel(parts)
			dataA, dataB := byLabel[aclPartLabelUsrA], byLabel[aclPartLabelUsrB]
			hashA, hashB := byLabel[aclPartLabelHashA], byLabel[aclPartLabelHashB]
			esp, root := byLabel[aclPartLabelEsp], byLabel[aclPartLabelRoot]
			require.NoError(t, shell.NewExecBuilder("mkfs.vfat", "-F", "32", esp.Path).Execute())
			require.NoError(t, shell.NewExecBuilder("mkfs.ext4", "-F", "-q", root.Path).Execute())
			require.NoError(t, shell.NewExecBuilder("mkfs.ext4", "-F", "-q", "-b", "4096", dataA.Path).Execute())
			require.NoError(t, shell.NewExecBuilder("mkfs.ext4", "-F", "-q", "-b", "4096", dataB.Path, "262144").Execute())

			settings := verityFormatSettings{
				hashAlgorithm: "sha256", dataBlockSizeBytes: 4096, hashBlockSizeBytes: 4096,
				dataSizeBytes: diskutils.GiB,
			}
			oldHashB, err := verityFormat(dev, dataB.Path, hashB.Path, false, 512, "usr", settings)
			require.NoError(t, err)
			inactiveHash := hashPartitionDigest(t, hashB.Path)
			beforeDump, _, err := shell.NewExecBuilder("sfdisk", "--dump", dev).ExecuteCaptureOutput()
			require.NoError(t, err)

			connection.partitionsLayout = []fstabEntryPartNum{
				{PartUuid: esp.PartUuid, FstabEntry: diskutils.FstabEntry{Target: "/boot"}},
				{PartUuid: root.PartUuid, FstabEntry: diskutils.FstabEntry{Target: "/"}},
				{PartUuid: dataA.PartUuid, FstabEntry: diskutils.FstabEntry{Target: "/usr"}, IsVerity: true},
			}
			ukiDir := filepath.Join(dir, UkiBuildDir)
			require.NoError(t, os.MkdirAll(ukiDir, 0o755))
			require.NoError(t, writeUkiKernelInfoFile(filepath.Join(ukiDir, UkiKernelInfoJson),
				map[string]UkiKernelInfo{"vmlinuz-test": {Cmdline: "root=LABEL=ROOT"}}))
			metadata := []verityDeviceMetadata{{
				name: "usr", dataPartUuid: dataA.PartUuid, hashPartUuid: hashA.PartUuid,
				dataDeviceMountIdType: imagecustomizerapi.MountIdentifierTypePartUuid,
				hashDeviceMountIdType: imagecustomizerapi.MountIdentifierTypePartUuid,
				corruptionOption:      imagecustomizerapi.CorruptionOptionIoError,
				formatSettings:        settings,
			}}
			metadata[0].formatSettings.dataSizeBytes = 2 * diskutils.GiB
			target := targetos.TargetOsAzureLinux3
			if test.acl {
				target.Distro = targetos.AzureContainerLinux
			}
			distro, err := NewDistroHandler(target)
			require.NoError(t, err)
			rc := &ResolvedConfig{Uki: &imagecustomizerapi.Uki{Mode: imagecustomizerapi.UkiModeCreate}}
			require.NoError(t, customizeVerityImage(context.Background(), dir, rc, connection, true, metadata, nil, distro))

			parts, err = diskutils.GetDiskPartitions(dev)
			require.NoError(t, err)
			after := partitionsByLabel(parts)
			assert.Equal(t, test.hashSize, after[aclPartLabelHashA].SizeInBytes)
			assert.Equal(t, uint64(17*diskutils.MiB), after[aclPartLabelHashB].SizeInBytes)
			assert.Equal(t, inactiveHash, hashPartitionDigest(t, hashB.Path))
			require.NoError(t, shell.NewExecBuilder("veritysetup", "verify", dataA.Path, hashA.Path, metadata[0].rootHash).Execute())
			require.NoError(t, shell.NewExecBuilder("veritysetup", "verify", dataB.Path, hashB.Path, oldHashB).Execute())
			if test.acl {
				afterDump, _, err := shell.NewExecBuilder("sfdisk", "--dump", dev).ExecuteCaptureOutput()
				require.NoError(t, err)
				assert.Equal(t, beforeDump, afterDump)
			}
			require.NoError(t, connection.CleanClose())

			output := filepath.Join(dir, "test.cosi")
			boot := &cosiapi.Bootloader{Type: cosiapi.BootloaderTypeSystemdBoot, SystemdBoot: &cosiapi.SystemDBoot{}}
			require.NoError(t, convertToCosi(dir, diskPath, output, connection.partitionsLayout, metadata,
				"ID=azurelinux\nVERSION_ID=3.0\n", nil, [16]byte{}, "", boot, 1, 0, false, nil))
			file, err := os.Open(output)
			require.NoError(t, err)
			defer file.Close()
			reader := tar.NewReader(file)
			var cosi cosiapi.MetadataJson
			found := false
			for range 3 {
				header, err := reader.Next()
				require.NoError(t, err)
				if header.Name == "metadata.json" {
					require.NoError(t, json.NewDecoder(reader).Decode(&cosi))
					found = true
					break
				}
			}
			require.True(t, found, "COSI metadata is missing")
			sizes := map[int]uint64{}
			for _, region := range cosi.Disk.GptRegions {
				if region.Number != nil {
					sizes[*region.Number] = region.Image.UncompressedSize
				}
			}
			assert.Equal(t, test.hashSize, sizes[3])
			assert.Equal(t, uint64(17*diskutils.MiB), sizes[5])
		})
	}
}

func hashPartitionDigest(t *testing.T, path string) []byte {
	t.Helper()
	file, err := os.Open(path)
	require.NoError(t, err)
	hash := sha256.New()
	_, err = io.Copy(hash, file)
	require.NoError(t, err)
	require.NoError(t, file.Close())
	return hash.Sum(nil)
}

func TestAclVerityRejectsUnequalHashSlots(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test must be run as root")
	}
	dir := t.TempDir()
	diskPath, _ := buildSyntheticAclDiskWithHashPartitions(t, dir)
	connection := &osImageConnection{imageConnection: imageconnection.NewImageConnection()}
	require.NoError(t, connection.imageConnection.ConnectLoopback(diskPath))
	defer connection.Close()
	dev := connection.Loopback().DevicePath()
	parts, err := diskutils.GetDiskPartitions(dev)
	require.NoError(t, err)
	byLabel := partitionsByLabel(parts)
	hashA, hashB := byLabel[aclPartLabelHashA], byLabel[aclPartLabelHashB]
	require.NoError(t, diskutils.ResizePartition(hashB.Path, dev, (diskutils.MiB-512)/512))
	before := hashPartitionDigest(t, hashA.Path)
	target := targetos.TargetOsAzureLinux3
	target.Distro = targetos.AzureContainerLinux
	distro, err := NewDistroHandler(target)
	require.NoError(t, err)
	metadata := []verityDeviceMetadata{{
		name: "usr", dataPartUuid: byLabel[aclPartLabelUsrA].PartUuid, hashPartUuid: hashA.PartUuid,
	}}
	err = customizeVerityImage(context.Background(), dir, &ResolvedConfig{}, connection, true, metadata, nil, distro)
	require.ErrorIs(t, err, ErrAclGrowUnexpectedLayout)
	assert.Equal(t, before, hashPartitionDigest(t, hashA.Path), "invalid geometry must fail before writing the hash")
	require.NoError(t, connection.CleanClose())
}

func TestAclInlineVerityKeepsPartitionGeometry(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test must be run as root")
	}
	dir := t.TempDir()
	diskPath, _, _ := buildSyntheticAclDisk(t, dir, 32)
	connection := &osImageConnection{imageConnection: imageconnection.NewImageConnection()}
	require.NoError(t, connection.imageConnection.ConnectLoopback(diskPath))
	defer connection.Close()
	dev := connection.Loopback().DevicePath()
	parts, err := diskutils.GetDiskPartitions(dev)
	require.NoError(t, err)
	byLabel := partitionsByLabel(parts)
	data, esp := byLabel[aclPartLabelUsrA], byLabel[aclPartLabelEsp]
	connection.partitionsLayout = []fstabEntryPartNum{
		{PartUuid: esp.PartUuid, FstabEntry: diskutils.FstabEntry{Target: "/boot"}},
	}
	ukiDir := filepath.Join(dir, UkiBuildDir)
	require.NoError(t, os.MkdirAll(ukiDir, 0o755))
	require.NoError(t, writeUkiKernelInfoFile(filepath.Join(ukiDir, UkiKernelInfoJson),
		map[string]UkiKernelInfo{"vmlinuz-test": {Cmdline: "root=LABEL=ROOT"}}))
	dataSize, err := imagecustomizerapi.CalculateInlineVerityDataSize(data.SizeInBytes)
	require.NoError(t, err)
	metadata := []verityDeviceMetadata{{
		name: "usr", dataPartUuid: data.PartUuid, hashPartUuid: data.PartUuid,
		dataDeviceMountIdType: imagecustomizerapi.MountIdentifierTypePartUuid,
		hashDeviceMountIdType: imagecustomizerapi.MountIdentifierTypePartUuid,
		corruptionOption:      imagecustomizerapi.CorruptionOptionIoError,
		formatSettings: verityFormatSettings{
			hashAlgorithm: "sha256", dataBlockSizeBytes: 4096, hashBlockSizeBytes: 4096,
			dataSizeBytes: dataSize, hashOffsetBytes: dataSize,
		},
	}}
	target := targetos.TargetOsAzureLinux3
	target.Distro = targetos.AzureContainerLinux
	distro, err := NewDistroHandler(target)
	require.NoError(t, err)
	before, _, err := shell.NewExecBuilder("sfdisk", "--dump", dev).ExecuteCaptureOutput()
	require.NoError(t, err)
	rc := &ResolvedConfig{Uki: &imagecustomizerapi.Uki{Mode: imagecustomizerapi.UkiModeCreate}}
	require.NoError(t, customizeVerityImage(context.Background(), dir, rc, connection, true, metadata, nil, distro))
	after, _, err := shell.NewExecBuilder("sfdisk", "--dump", dev).ExecuteCaptureOutput()
	require.NoError(t, err)
	assert.Equal(t, before, after)
	require.NoError(t, shell.NewExecBuilder("veritysetup", "verify", fmt.Sprintf("--hash-offset=%d", dataSize),
		data.Path, data.Path, metadata[0].rootHash).Execute())
	require.NoError(t, connection.CleanClose())
}
