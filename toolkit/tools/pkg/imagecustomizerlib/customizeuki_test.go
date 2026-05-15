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
	for _, baseImageInfo := range baseImageAzureLinuxCoreEfiAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageVerityUsrUkiHelper(t, baseImageInfo)
		})
	}
}

func testCustomizeImageVerityUsrUkiHelper(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	ukifyExists, err := file.CommandExists("ukify")
	assert.NoError(t, err)
	if !ukifyExists {
		t.Skip("The 'ukify' command is not available")
	}

	if runtime.GOARCH == "arm64" {
		t.Skip("systemd-boot not available on AZL3 ARM64 yet")
	}

	testTempDir := filepath.Join(tmpDir, fmt.Sprintf("TestCustomizeImageUsrVerityUki_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	configFile := filepath.Join(testDir, verityUsrUkiConfigFile(t, baseImageInfo))

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
	for _, baseImageInfo := range baseImageAzureLinuxCoreEfiAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageVerityUsrUkiRecustomizeHelper(t, baseImageInfo)
		})
	}
}

func testCustomizeImageVerityUsrUkiRecustomizeHelper(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	ukifyExists, err := file.CommandExists("ukify")
	assert.NoError(t, err)
	if !ukifyExists {
		t.Skip("The 'ukify' command is not available")
	}

	if runtime.GOARCH == "arm64" {
		t.Skip("systemd-boot not available on AZL3 ARM64 yet")
	}

	testTempDir := filepath.Join(tmpDir, fmt.Sprintf("TestCustomizeImageUsrVerityUkiRecustomize_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	configFile := filepath.Join(testDir, verityUsrUkiConfigFile(t, baseImageInfo))

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
	for _, baseImageInfo := range baseImageAzureLinuxCoreEfiAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageVerityUsrUkiPassthroughHelper(t, baseImageInfo)
		})
	}
}

func testCustomizeImageVerityUsrUkiPassthroughHelper(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	ukifyExists, err := file.CommandExists("ukify")
	assert.NoError(t, err)
	if !ukifyExists {
		t.Skip("The 'ukify' command is not available")
	}

	if runtime.GOARCH == "arm64" {
		t.Skip("systemd-boot not available on AZL3 ARM64 yet")
	}

	testTempDir := filepath.Join(tmpDir, fmt.Sprintf("TestCustomizeImageUsrVerityUkiPassthrough_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	configFile := filepath.Join(testDir, verityUsrUkiConfigFile(t, baseImageInfo))

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
	for _, baseImageInfo := range baseImageAzureLinuxCoreEfiAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageVerityRootUkiHelper(t, baseImageInfo)
		})
	}
}

func testCustomizeImageVerityRootUkiHelper(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	ukifyExists, err := file.CommandExists("ukify")
	assert.NoError(t, err)
	if !ukifyExists {
		t.Skip("The 'ukify' command is not available")
	}

	if runtime.GOARCH == "arm64" {
		t.Skip("systemd-boot not available on AZL3 ARM64 yet")
	}

	testTempDir := filepath.Join(tmpDir, fmt.Sprintf("TestCustomizeImageRootVerityUki_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	configFile := filepath.Join(testDir, verityRootUkiConfigFile(t, baseImageInfo))

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
		"PARTUUID="+partitions[4].PartUuid, "usr", buildDir, "rd.info", "panic-on-corruption", false /*inlineVerity*/)

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
		"PARTUUID="+partitions[4].PartUuid, "root", buildDir, "", "panic-on-corruption", false /*inlineVerity*/)

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

// verityUsrUkiConfigFile returns the verity-usr-uki test config file appropriate for the
// given base image version (azl3 vs azl4) and host architecture.
func verityUsrUkiConfigFile(t *testing.T, baseImageInfo testBaseImageInfo) string {
	switch baseImageInfo.Version {
	case baseImageVersionAzl3:
		return "verity-usr-uki-azl3.yaml"
	case baseImageVersionAzl4:
		return fmt.Sprintf("verity-usr-uki-%s-azl4.yaml", runtime.GOARCH)
	default:
		t.Fatalf("unsupported base image version for verity-usr-uki test: %s", baseImageInfo.Version)
		return ""
	}
}

// verityRootUkiConfigFile returns the verity-root-uki test config file appropriate for the
// given base image version (azl3 vs azl4) and host architecture.
func verityRootUkiConfigFile(t *testing.T, baseImageInfo testBaseImageInfo) string {
	switch baseImageInfo.Version {
	case baseImageVersionAzl3:
		return "verity-root-uki-azl3.yaml"
	case baseImageVersionAzl4:
		return fmt.Sprintf("verity-root-uki-%s-azl4.yaml", runtime.GOARCH)
	default:
		t.Fatalf("unsupported base image version for verity-root-uki test: %s", baseImageInfo.Version)
		return ""
	}
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

func TestGetKernelNameFromUki(t *testing.T) {
	tests := []struct {
		name        string
		ukiPath     string
		expected    string
		expectError bool
	}{
		{
			name:     "standard vmlinuz naming",
			ukiPath:  "/boot/efi/EFI/Linux/vmlinuz-6.6.51.1-5.azl3.efi",
			expected: "vmlinuz-6.6.51.1-5.azl3",
		},
		{
			name:     "non-standard naming (ACL)",
			ukiPath:  "/boot/EFI/Linux/acl.efi",
			expected: "acl",
		},
		{
			name:     "non-standard naming with path",
			ukiPath:  "/some/path/custom-kernel.efi",
			expected: "custom-kernel",
		},
		{
			name:        "no .efi extension",
			ukiPath:     "/boot/EFI/Linux/vmlinuz-6.6.51",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getKernelNameFromUki(tt.ukiPath)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestReadKernelCmdlinesFromBLSEntries(t *testing.T) {
	tests := []struct {
		name          string
		files         map[string]string
		wantKernels   []string
		wantArgsFor   map[string][]string // kernel -> expected arg strings (Arg field) in order
		wantErrSubstr string
	}{
		{
			name: "preserves quoted value with embedded space in options",
			files: map[string]string{
				"azl.conf": "title Azure Linux\n" +
					"linux /vmlinuz-6.6\n" +
					"options root=/dev/sda1 rd.cmdline=\"foo bar\" quiet\n",
			},
			wantKernels: []string{"vmlinuz-6.6"},
			wantArgsFor: map[string][]string{
				"vmlinuz-6.6": {"root=/dev/sda1", "rd.cmdline=foo bar", "quiet"},
			},
		},
		{
			name: "tab between key and value is recognized",
			files: map[string]string{
				"azl.conf": "title Azure Linux\n" +
					"linux\t/vmlinuz-6.6\n" +
					"options\troot=/dev/sda1 quiet\n",
			},
			wantKernels: []string{"vmlinuz-6.6"},
			wantArgsFor: map[string][]string{
				"vmlinuz-6.6": {"root=/dev/sda1", "quiet"},
			},
		},
		{
			name: "mixed tab-then-space separator is recognized",
			files: map[string]string{
				"azl.conf": "title Azure Linux\n" +
					"linux /vmlinuz-6.6\n" +
					"options\troot=/dev/sda1 quiet\n",
			},
			wantKernels: []string{"vmlinuz-6.6"},
			wantArgsFor: map[string][]string{
				"vmlinuz-6.6": {"root=/dev/sda1", "quiet"},
			},
		},
		{
			name: "multiple options lines are concatenated per BLS spec",
			files: map[string]string{
				"azl.conf": "title Azure Linux\n" +
					"linux /vmlinuz-6.6\n" +
					"options root=/dev/sda1\n" +
					"options quiet rhgb\n",
			},
			wantKernels: []string{"vmlinuz-6.6"},
			wantArgsFor: map[string][]string{
				"vmlinuz-6.6": {"root=/dev/sda1", "quiet", "rhgb"},
			},
		},
		{
			name: "recovery entries are skipped, not errored",
			files: map[string]string{
				"normal.conf": "title Azure Linux\n" +
					"linux /vmlinuz-6.6\n" +
					"options root=/dev/sda1\n",
				"rescue.conf": "title Azure Linux (recovery)\n" +
					"linux /vmlinuz-6.6-rescue\n" +
					"options root=/dev/sda1 systemd.unit=rescue.target\n",
			},
			wantKernels: []string{"vmlinuz-6.6"},
			wantArgsFor: map[string][]string{
				"vmlinuz-6.6": {"root=/dev/sda1"},
			},
		},
		{
			name: "comments and blank lines are tolerated",
			files: map[string]string{
				"azl.conf": "# An Azure Linux BLS entry\n" +
					"\n" +
					"title Azure Linux\n" +
					"linux /vmlinuz-6.6\n" +
					"# kernel command line:\n" +
					"options root=/dev/sda1 quiet\n",
			},
			wantKernels: []string{"vmlinuz-6.6"},
			wantArgsFor: map[string][]string{
				"vmlinuz-6.6": {"root=/dev/sda1", "quiet"},
			},
		},
		{
			name: "'efi' key produces an error",
			files: map[string]string{
				"efi.conf": "title Azure Linux\n" +
					"efi /EFI/Linux/vmlinuz.efi\n",
			},
			wantErrSubstr: "uses 'efi' key",
		},
		{
			name: "'uki' key produces an error",
			files: map[string]string{
				"uki.conf": "title Azure Linux\n" +
					"uki /EFI/Linux/vmlinuz.efi\n",
			},
			wantErrSubstr: "uses 'uki' key",
		},
		{
			name: "'uki-url' key produces an error",
			files: map[string]string{
				"uki-url.conf": "title Azure Linux\n" +
					"uki-url https://example.com/vmlinuz.efi\n",
			},
			wantErrSubstr: "uses 'uki-url' key",
		},
		{
			name: "no 'linux' key produces an error",
			files: map[string]string{
				"bad.conf": "title Azure Linux\n" +
					"options root=/dev/sda1\n",
			},
			wantErrSubstr: "missing 'linux' key",
		},
		{
			name: "no 'title' key is treated as a normal entry",
			files: map[string]string{
				"bad.conf": "linux /vmlinuz-6.6\n" +
					"options root=/dev/sda1\n",
			},
			wantKernels: []string{"vmlinuz-6.6"},
			wantArgsFor: map[string][]string{
				"vmlinuz-6.6": {"root=/dev/sda1"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bootDir := t.TempDir()
			entriesDir := filepath.Join(bootDir, "loader", "entries")
			err := os.MkdirAll(entriesDir, 0o755)
			if !assert.NoError(t, err) {
				return
			}
			for name, content := range tc.files {
				err := os.WriteFile(filepath.Join(entriesDir, name), []byte(content), 0o644)
				if !assert.NoError(t, err) {
					return
				}
			}

			got, err := readKernelCmdlinesFromBLSEntries(bootDir)
			if tc.wantErrSubstr != "" {
				if assert.Error(t, err) {
					assert.Contains(t, err.Error(), tc.wantErrSubstr)
				}
				return
			}

			if !assert.NoError(t, err) {
				return
			}

			gotKernels := make([]string, 0, len(got))
			for k := range got {
				gotKernels = append(gotKernels, k)
			}
			assert.ElementsMatch(t, tc.wantKernels, gotKernels)

			for kernel, wantArgs := range tc.wantArgsFor {
				args, ok := got[kernel]
				if !assert.True(t, ok, "expected kernel %q in result", kernel) {
					continue
				}
				gotArgStrings := make([]string, 0, len(args))
				for _, arg := range args {
					gotArgStrings = append(gotArgStrings, arg.Arg)
				}
				assert.Equal(t, wantArgs, gotArgStrings, "args for kernel %q", kernel)
			}
		})
	}
}
