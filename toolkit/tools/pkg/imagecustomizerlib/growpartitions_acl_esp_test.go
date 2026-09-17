// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildSyntheticAclDisk creates a raw disk file with ACL's exact 5-partition layout (EFI-SYSTEM,
// USR-A, USR-B, OEM, ROOT) and a real vfat ESP containing a marker file. It returns the disk path
// and the marker's relative path/content so callers can assert preservation across a grow.
func buildSyntheticAclDisk(t *testing.T, dir string, espSizeMiB int) (diskPath, markerName, markerContent string) {
	diskPath = filepath.Join(dir, "acl-synth.raw")
	// ESP + USR-A(8M) + USR-B(8M) + OEM(4M) + ROOT(16M) + slack.
	totalMiB := espSizeMiB + 8 + 8 + 4 + 16 + 8
	require.NoError(t, diskutils.CreateSparseDisk(diskPath, uint64(totalMiB), 0o644))

	loop, err := safeloopback.NewLoopback(diskPath)
	require.NoError(t, err)
	defer loop.Close()
	dev := loop.DevicePath()

	// ACL's exact labels + the USR type GUID + A/B attr bits, so readAclPartitionTable accepts it.
	usrType := "5DFBF5F4-2848-4BAC-AA5E-0D9A20B745A6"
	sfScript := "" +
		"label: gpt\n" +
		"unit: sectors\n\n" +
		"size=" + strconv.Itoa(espSizeMiB*2048) + ", type=C12A7328-F81F-11D2-BA4B-00A0C93EC93B, name=\"EFI-SYSTEM\"\n" +
		"size=" + strconv.Itoa(8*2048) + ", type=" + usrType + ", name=\"USR-A\", attrs=\"GUID:48,56\"\n" +
		"size=" + strconv.Itoa(8*2048) + ", type=" + usrType + ", name=\"USR-B\", attrs=\"GUID:48,56\"\n" +
		"size=" + strconv.Itoa(4*2048) + ", type=0FC63DAF-8483-4772-8E79-3D69D8477DE4, name=\"OEM\"\n" +
		"type=0FC63DAF-8483-4772-8E79-3D69D8477DE4, name=\"ROOT\"\n"
	require.NoError(t, shell.NewExecBuilder("sfdisk", dev).Stdin(sfScript).
		LogLevel(logrus.DebugLevel, logrus.WarnLevel).Execute())
	require.NoError(t, diskutils.RefreshPartitions(dev))

	parts, err := diskutils.GetDiskPartitions(dev)
	require.NoError(t, err)
	esp, ok := partitionsByLabel(parts)[aclPartLabelEsp]
	require.True(t, ok, "synthetic ESP not found")

	// Real vfat on the ESP with a known label + a marker file.
	require.NoError(t, shell.NewExecBuilder("mkfs.vfat", "-F", "32", "-n", "EFI-SYSTEM", esp.Path).
		LogLevel(logrus.DebugLevel, logrus.WarnLevel).Execute())

	markerName = "marker.txt"
	markerContent = "acl-esp-grow-preserved\n"
	mntDir := filepath.Join(dir, "espmnt")
	mnt, err := safemount.NewMount(esp.Path, mntDir, "vfat", 0, "", true)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(mntDir, markerName), []byte(markerContent), 0o644))
	require.NoError(t, mnt.CleanClose())

	require.NoError(t, loop.CleanClose())
	return diskPath, markerName, markerContent
}

// TestGrowAclEspFilesystemPreservesContent is a regression test for the ESP-grow path
// (recreateAclEspFilesystem). Growing ACL's ESP must (a) succeed and (b) preserve the ESP's files.
// The path previously staged files from the new (empty) ESP instead of the base ESP, so any real
// grow failed at mount ("invalid argument"). Requires root + loopback + mkfs.vfat.
func TestGrowAclEspFilesystemPreservesContent(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test must be run as root because it uses loopback devices and mounts filesystems")
	}

	dir := t.TempDir()
	baseEspMiB := 32
	grownEspMiB := 96

	basePath, markerName, markerContent := buildSyntheticAclDisk(t, dir, baseEspMiB)

	acl := &imagecustomizerapi.Acl{
		Esp: &imagecustomizerapi.AclPartitionGrow{
			Size: imagecustomizerapi.DiskSize(grownEspMiB * diskutils.MiB),
		},
	}

	grownPath := filepath.Join(dir, "acl-grown.raw")
	err := growAclStandardPartitions(context.Background(), acl, basePath, grownPath)
	require.NoError(t, err, "ESP grow must succeed")

	// Verify the grown ESP: partition enlarged and the marker file preserved.
	loop, err := safeloopback.NewLoopback(grownPath)
	require.NoError(t, err)
	defer loop.Close()
	require.NoError(t, diskutils.RefreshPartitions(loop.DevicePath()))

	parts, err := diskutils.GetDiskPartitions(loop.DevicePath())
	require.NoError(t, err)
	esp, ok := partitionsByLabel(parts)[aclPartLabelEsp]
	require.True(t, ok, "grown ESP not found")

	// The ESP partition must now be (about) the requested size (allow rounding).
	assert.GreaterOrEqual(t, esp.SizeInBytes, uint64((grownEspMiB-1)*diskutils.MiB),
		"ESP partition should be grown to ~%dMiB", grownEspMiB)

	mntDir := filepath.Join(dir, "grownmnt")
	mnt, err := safemount.NewMount(esp.Path, mntDir, "vfat", 0, "", true)
	require.NoError(t, err, "grown ESP must be a valid, mountable vfat")
	defer mnt.Close()

	got, err := os.ReadFile(filepath.Join(mntDir, markerName))
	require.NoError(t, err, "marker file must survive the grow")
	assert.Equal(t, markerContent, string(got), "marker content must be preserved")

	require.NoError(t, mnt.CleanClose())
	require.NoError(t, loop.CleanClose())
}
