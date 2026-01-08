// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
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
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	configFile := filepath.Join(testDir, "verity-usr-uki.yaml")

	// Customize image.
	err = CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	ukiFilesChecksums, _, ok := verifyUsrVerity(t, buildDir, outImageFilePath, nil, nil)
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

	verifyUsrVerity(t, buildDir, outImageFilePath2, ukiFilesChecksums, nil)
}

func TestCustomizeImageVerityUsrUkiRecustomize(t *testing.T) {
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

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageUsrVerityUkiRecustomize")
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	configFile := filepath.Join(testDir, "verity-usr-uki.yaml")

	err = CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	ukiFilesChecksums, addonFilesChecksums, ok := verifyUsrVerity(t, buildDir, outImageFilePath, nil, nil)
	if !ok {
		return
	}

	outImageFilePath2 := filepath.Join(testTempDir, "image2.raw")
	configFile2 := filepath.Join(testDir, "verity-reinit-usr-uki.yaml")

	err = CustomizeImageWithConfigFile(t.Context(), buildDir, configFile2, outImageFilePath, nil, outImageFilePath2, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	newUkiFilesChecksums, newAddonFilesChecksums, ok := verifyUsrVerity(t, buildDir, outImageFilePath2, nil, nil)
	if !ok {
		return
	}

	// With UKI addon architecture:
	// - Main UKI (kernel + os-release + initramfs) should NOT change when only verity is re-initialized
	// - Addon (cmdline with verity hashes) SHOULD change when verity is re-initialized
	for ukiFile := range ukiFilesChecksums {
		oldChecksum := ukiFilesChecksums[ukiFile]
		newChecksum, exists := newUkiFilesChecksums[ukiFile]
		assert.True(t, exists, "UKI file should exist after re-customization: %s", ukiFile)
		// Main UKI should stay the same since initramfs doesn't change
		assert.Equal(t, oldChecksum, newChecksum, "Main UKI checksum should NOT change (only cmdline in addon changes): %s", ukiFile)
	}

	// Addon checksums should change because verity hashes in cmdline change
	for addonFile, oldAddonChecksum := range addonFilesChecksums {
		newAddonChecksum, exists := newAddonFilesChecksums[addonFile]
		assert.True(t, exists, "Addon file should exist after re-customization: %s", addonFile)
		assert.NotEqual(t, oldAddonChecksum, newAddonChecksum, "Addon checksum should change after verity re-initialization: %s", addonFile)
	}

	mountPoints := []testutils.MountPoint{
		{
			PartitionNum:   1,
			Path:           "/boot/efi",
			FileSystemType: "vfat",
		},
		{
			PartitionNum:   2,
			Path:           "/boot",
			FileSystemType: "ext4",
		},
		{
			PartitionNum:   5,
			Path:           "/",
			FileSystemType: "ext4",
		},
	}

	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath2, false /*includeDefaultMounts*/, mountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	bootPath := filepath.Join(imageConnection.Chroot().RootDir(), "/boot")
	bootEntries, err := os.ReadDir(bootPath)
	assert.NoError(t, err)

	// /boot should only contain "efi" and optionally "lost+found" directories.
	assert.LessOrEqual(t, len(bootEntries), 2, "/boot should contain at most 2 entries: 'efi' and 'lost+found'")

	// Verify all entries are either "efi" or "lost+found"
	hasEfi := false
	for _, entry := range bootEntries {
		assert.True(t, entry.Name() == "efi" || entry.Name() == "lost+found",
			"unexpected entry in /boot: %s (expected only 'efi' and 'lost+found')", entry.Name())
		assert.True(t, entry.IsDir(), "%s should be a directory", entry.Name())
		if entry.Name() == "efi" {
			hasEfi = true
		}
	}
	assert.True(t, hasEfi, "/boot must contain the 'efi' directory")
}

func TestCustomizeImageVerityUsrUkiPassthrough(t *testing.T) {
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

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageUsrVerityUkiPassthrough")
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	configFile := filepath.Join(testDir, "verity-usr-uki.yaml")

	err = CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	ukiFilesChecksums, _, ok := verifyUsrVerity(t, buildDir, outImageFilePath, nil, nil)
	if !ok {
		return
	}

	outImageFilePath2 := filepath.Join(testTempDir, "image-passthrough.raw")
	configFile2 := filepath.Join(testDir, "verity-usr-uki-passthrough.yaml")

	err = CustomizeImageWithConfigFile(t.Context(), buildDir, configFile2, outImageFilePath, nil, outImageFilePath2, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	passthroughUkiChecksums, _, ok := verifyUsrVerity(t, buildDir, outImageFilePath2, ukiFilesChecksums, nil)
	if !ok {
		return
	}

	// Verify UKI checksums are unchanged.
	for ukiFile := range ukiFilesChecksums {
		originalChecksum := ukiFilesChecksums[ukiFile]
		passthroughChecksum, exists := passthroughUkiChecksums[ukiFile]
		assert.True(t, exists, "UKI file should exist after passthrough customization: %s", ukiFile)
		assert.Equal(t, originalChecksum, passthroughChecksum,
			"UKI checksum MUST NOT change in passthrough mode: %s", ukiFile)
	}
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
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	configFile := filepath.Join(testDir, "verity-root-uki.yaml")

	// Customize image.
	err = CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	_, ok := verifyRootVerityUki(t, buildDir, outImageFilePath, nil)
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

	verifyRootVerityUki(t, buildDir, outImageFilePath2, nil)
}

func verifyUsrVerity(t *testing.T, buildDir string, imagePath string,
	expectedUkiFilesChecksums map[string]string, expectedAddonFilesChecksums map[string]string,
) (map[string]string, map[string]string, bool) {
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
		return nil, nil, false
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

	// Verify fstab entries
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

	// Get UKI files and calculate checksums
	ukiFiles, err := getUkiFiles(espPath)
	if !assert.NoError(t, err) {
		return nil, nil, false
	}

	ukiFilesChecksums := calculateUkiFileChecksums(t, ukiFiles)
	if ukiFilesChecksums == nil {
		return nil, nil, false
	}

	if expectedUkiFilesChecksums != nil {
		// Verify that the UKI files haven't changed.
		// Note: This indirectly also checks that the verity partitions haven't changed since the UKIs contain the
		// verity root hash in the kernel command-line args.
		assert.Equal(t, expectedUkiFilesChecksums, ukiFilesChecksums)
	}

	// Get addon files and calculate checksums
	addonFiles, err := getUkiAddonFiles(espPath)
	if !assert.NoError(t, err) {
		return nil, nil, false
	}

	addonFilesChecksums := calculateUkiFileChecksums(t, addonFiles)
	if addonFilesChecksums == nil {
		return nil, nil, false
	}

	if expectedAddonFilesChecksums != nil {
		assert.Equal(t, expectedAddonFilesChecksums, addonFilesChecksums, "Addon checksums should match expected")
	}

	err = imageConnection.CleanClose()
	if !assert.NoError(t, err) {
		return nil, nil, false
	}

	return ukiFilesChecksums, addonFilesChecksums, true
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
		"PARTUUID="+partitions[4].PartUuid, "root", buildDir, "", "panic-on-corruption")

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

	err = imageConnection.CleanClose()
	if !assert.NoError(t, err) {
		return nil, false
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

// getUkiAddonFiles returns a list of UKI addon files (.addon.efi) in the ESP partition.
func getUkiAddonFiles(espPath string) ([]string, error) {
	espLinuxPath := filepath.Join(espPath, "EFI/Linux")
	addonDirs, err := filepath.Glob(filepath.Join(espLinuxPath, "vmlinuz-*.efi.extra.d"))
	if err != nil {
		return nil, fmt.Errorf("failed to search for UKI addon directories in ESP partition:\n%w", err)
	}

	var addonFiles []string
	for _, addonDir := range addonDirs {
		addons, err := filepath.Glob(filepath.Join(addonDir, "*.addon.efi"))
		if err != nil {
			return nil, fmt.Errorf("failed to search for addon files in %s:\n%w", addonDir, err)
		}
		addonFiles = append(addonFiles, addons...)
	}

	return addonFiles, nil
}
