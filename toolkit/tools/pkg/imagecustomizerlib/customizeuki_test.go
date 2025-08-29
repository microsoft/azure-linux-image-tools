// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/testutils"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

func TestCustomizeImageVerityUsrUki(t *testing.T) {
	baseImageInfo := testBaseImageAzl3CoreEfi
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	ukifyExists, err := file.CommandExists("ukify")
	assert.NoError(t, err)
	if !ukifyExists {
		t.Skip("The 'ukify' command is not available")
	}

	if runtime.GOARCH == "arm64" {
		t.Skip("systemd-boot not available on AZL3 ARM64 yet")
	}

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageUsrVerityUki")
	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	configFile := filepath.Join(testDir, "verity-usr-uki.yaml")

	// Customize image.
	err = CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	ukiFilesChecksums, ok := verifyUsrVerity(t, buildDir, outImageFilePath, nil)
	if !ok {
		return
	}

	// Customize again without changing /usr verity or the UKI.
	outImageFilePath2 := filepath.Join(testTempDir, "image2.raw")
	configFile2 := filepath.Join(testDir, "verity-reinit-usr-nop.yaml")

	err = CustomizeImageWithConfigFile(t.Context(), buildDir, configFile2, outImageFilePath, nil, outImageFilePath2, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	verifyUsrVerity(t, buildDir, outImageFilePath2, ukiFilesChecksums)
}

func TestCustomizeImageVerityRootUki(t *testing.T) {
	baseImageInfo := testBaseImageAzl3CoreEfi
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	ukifyExists, err := file.CommandExists("ukify")
	assert.NoError(t, err)
	if !ukifyExists {
		t.Skip("The 'ukify' command is not available")
	}

	if runtime.GOARCH == "arm64" {
		t.Skip("systemd-boot not available on AZL3 ARM64 yet")
	}

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageRootVerityUki")
	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	configFile := filepath.Join(testDir, "verity-root-uki.yaml")

	// Customize image.
	err = CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	ukiFilesChecksums, ok := verifyRootVerityUki(t, buildDir, outImageFilePath, nil)
	if !ok {
		return
	}

	// Customize again without changing /root verity or the UKI.
	outImageFilePath2 := filepath.Join(testTempDir, "image2.raw")
	configFile2 := filepath.Join(testDir, "verity-reinit-root-nop.yaml")

	err = CustomizeImageWithConfigFile(t.Context(), buildDir, configFile2, outImageFilePath, nil, outImageFilePath2, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	verifyRootVerityUki(t, buildDir, outImageFilePath2, ukiFilesChecksums)
}

func verifyUsrVerity(t *testing.T, buildDir string, imagePath string, expectedUkiFilesChecksums map[string]string,
) (map[string]string, bool) {
	// Connect to customized image.
	mountPoints := []testutils.MountPoint{
		{
			PartitionNum:   5,
			Path:           "/",
			FileSystemType: "ext4",
		},
		{
			PartitionNum:   2,
			Path:           "/boot",
			FileSystemType: "ext4",
		},
		{
			PartitionNum:   1,
			Path:           "/boot/efi",
			FileSystemType: "vfat",
		},
		{
			PartitionNum:   3,
			Path:           "/usr",
			FileSystemType: "ext4",
			Flags:          unix.MS_RDONLY,
		},
		{
			PartitionNum:   6,
			Path:           "/var",
			FileSystemType: "ext4",
		},
	}

	imageConnection, err := testutils.ConnectToImage(buildDir, imagePath, false /*includeDefaultMounts*/, mountPoints)
	if !assert.NoError(t, err) {
		return nil, false
	}
	defer imageConnection.Close()

	partitions, err := getDiskPartitionsMap(imageConnection.Loopback().DevicePath())
	assert.NoError(t, err, "get disk partitions")

	// Verify that verity is configured correctly.
	espPath := filepath.Join(imageConnection.Chroot().RootDir(), "/boot/efi")
	usrDevice := testutils.PartitionDevPath(imageConnection, 3)
	usrHashDevice := testutils.PartitionDevPath(imageConnection, 4)
	verifyVerityUki(t, espPath, usrDevice, usrHashDevice, "PARTUUID="+partitions[3].PartUuid,
		"PARTUUID="+partitions[4].PartUuid, "usr", buildDir, "rd.info", "panic-on-corruption")

	expectedFstabEntries := []diskutils.FstabEntry{
		{
			Source:     "PARTUUID=" + partitions[5].PartUuid,
			Target:     "/",
			FsType:     "ext4",
			Options:    "noexec",
			VfsOptions: 0x8,
			FsOptions:  "",
			Freq:       0,
			PassNo:     1,
		},
		{
			Source:     "PARTUUID=" + partitions[2].PartUuid,
			Target:     "/boot",
			FsType:     "ext4",
			Options:    "defaults",
			VfsOptions: 0x0,
			FsOptions:  "",
			Freq:       0,
			PassNo:     2,
		},
		{
			Source:     "PARTUUID=" + partitions[1].PartUuid,
			Target:     "/boot/efi",
			FsType:     "vfat",
			Options:    "umask=0077",
			VfsOptions: 0x0,
			FsOptions:  "umask=0077",
			Freq:       0,
			PassNo:     2,
		},
		{
			Source:     "/dev/mapper/usr",
			Target:     "/usr",
			FsType:     "ext4",
			Options:    "ro",
			VfsOptions: 0x1,
			FsOptions:  "",
			Freq:       0,
			PassNo:     2,
		},
		{
			Source:     "PARTUUID=" + partitions[6].PartUuid,
			Target:     "/var",
			FsType:     "ext4",
			Options:    "defaults",
			VfsOptions: 0x0,
			FsOptions:  "",
			Freq:       0,
			PassNo:     2,
		},
	}
	filteredFstabEntries := getFilteredFstabEntries(t, imageConnection)
	assert.Equal(t, expectedFstabEntries, filteredFstabEntries)

	ukiFiles, err := getUkiFiles(espPath)
	if !assert.NoError(t, err) {
		return nil, false
	}

	ukiFilesChecksums := calculateUkiFileChecksums(t, ukiFiles)
	if ukiFilesChecksums == nil {
		return nil, false
	}

	if expectedUkiFilesChecksums != nil {
		// Verify that the UKI files haven't changed.
		// Note: This indirectly also checks that the verity partitions haven't changed since the UKIs contain the
		// verity root hash in the kernel command-line args.
		assert.Equal(t, expectedUkiFilesChecksums, ukiFilesChecksums)
	}

	return ukiFilesChecksums, true
}

func verifyRootVerityUki(t *testing.T, buildDir string, imagePath string, expectedUkiFilesChecksums map[string]string,
) (map[string]string, bool) {
	// Connect to customized image.
	mountPoints := []testutils.MountPoint{
		{
			PartitionNum:   3,
			Path:           "/",
			FileSystemType: "ext4",
			Flags:          unix.MS_RDONLY,
		},
		{
			PartitionNum:   2,
			Path:           "/boot",
			FileSystemType: "ext4",
		},
		{
			PartitionNum:   1,
			Path:           "/boot/efi",
			FileSystemType: "vfat",
		},
		{
			PartitionNum:   5,
			Path:           "/var",
			FileSystemType: "ext4",
		},
	}

	imageConnection, err := testutils.ConnectToImage(buildDir, imagePath, false /*includeDefaultMounts*/, mountPoints)
	if !assert.NoError(t, err) {
		return nil, false
	}
	defer imageConnection.Close()

	partitions, err := getDiskPartitionsMap(imageConnection.Loopback().DevicePath())
	assert.NoError(t, err, "get disk partitions")

	// Verify that verity is configured correctly.
	espPath := filepath.Join(imageConnection.Chroot().RootDir(), "/boot/efi")
	rootDevice := testutils.PartitionDevPath(imageConnection, 3)
	rootHashDevice := testutils.PartitionDevPath(imageConnection, 4)
	verifyVerityUki(t, espPath, rootDevice, rootHashDevice, "PARTUUID="+partitions[3].PartUuid,
		"PARTUUID="+partitions[4].PartUuid, "root", buildDir, "rd.info", "panic-on-corruption")

	expectedFstabEntries := []diskutils.FstabEntry{
		{
			Source:     "/dev/mapper/root",
			Target:     "/",
			FsType:     "ext4",
			Options:    "ro",
			VfsOptions: 0x1,
			FsOptions:  "",
			Freq:       0,
			PassNo:     1,
		},
		{
			Source:     "PARTUUID=" + partitions[2].PartUuid,
			Target:     "/boot",
			FsType:     "ext4",
			Options:    "defaults",
			VfsOptions: 0x0,
			FsOptions:  "",
			Freq:       0,
			PassNo:     2,
		},
		{
			Source:     "PARTUUID=" + partitions[1].PartUuid,
			Target:     "/boot/efi",
			FsType:     "vfat",
			Options:    "umask=0077",
			VfsOptions: 0x0,
			FsOptions:  "umask=0077",
			Freq:       0,
			PassNo:     2,
		},
		{
			Source:     "PARTUUID=" + partitions[5].PartUuid,
			Target:     "/var",
			FsType:     "ext4",
			Options:    "defaults",
			VfsOptions: 0x0,
			FsOptions:  "",
			Freq:       0,
			PassNo:     2,
		},
	}
	filteredFstabEntries := getFilteredFstabEntries(t, imageConnection)
	assert.Equal(t, expectedFstabEntries, filteredFstabEntries)

	ukiFiles, err := getUkiFiles(espPath)
	if !assert.NoError(t, err) {
		return nil, false
	}

	ukiFilesChecksums := calculateUkiFileChecksums(t, ukiFiles)
	if ukiFilesChecksums == nil {
		return nil, false
	}

	if expectedUkiFilesChecksums != nil {
		// Verify that the UKI files haven't changed.
		// Note: This indirectly also checks that the verity partitions haven't changed since the UKIs contain the
		// verity root hash in the kernel command-line args.
		assert.Equal(t, expectedUkiFilesChecksums, ukiFilesChecksums)
	}

	return ukiFilesChecksums, true
}

// calculateUkiFileChecksums generates SHA256 checksums for a list of UKI files.
// Returns nil if any error occurs during checksum calculation.
func calculateUkiFileChecksums(t *testing.T, ukiFiles []string) map[string]string {
	ukiFilesChecksums := make(map[string]string)
	for _, ukiFile := range ukiFiles {
		checksum, err := file.GenerateSHA256(ukiFile)
		if !assert.NoError(t, err) {
			return nil
		}
		ukiFilesChecksums[ukiFile] = checksum
	}

	return ukiFilesChecksums
}
