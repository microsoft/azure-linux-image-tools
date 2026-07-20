// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

const nulPaddingFillerArg = "rd.debug"

func TestCustomizeImageVerityUsrUki(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinux3Plus {
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

	toolsDir := testutils.GetDownloadedToolsDir(t, testutilsDir, baseImageInfo.Distro, baseImageInfo.Version)

	testTempDir := filepath.Join(tmpDir, fmt.Sprintf("TestCustomizeImageUsrVerityUki_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	configFile := filepath.Join(testDir, verityUsrUkiConfigFile(t, baseImageInfo))

	previewFeatures := []imagecustomizerapi.PreviewFeature{imagecustomizerapi.PreviewFeatureToolsDir}
	previewFeatures = append(previewFeatures, baseImageInfo.PreviewFeatures...)

	// --tools-dir exercises the toolsChroot path in isPackageInstalled used by UKI 'create' validation.
	err = CustomizeImageWithConfigFile(t.Context(), configFile, ImageCustomizerOptions{
		BuildDir:             buildDir,
		InputImageFile:       baseImage,
		OutputImageFile:      outImageFilePath,
		OutputImageFormat:    imagecustomizerapi.ImageFormatType("raw"),
		UseBaseImageRpmRepos: true,
		PreviewFeatures:      previewFeatures,
		SetFilesContext:      *setfilesContext,
		ToolsDir:             toolsDir,
	})
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

	err = basicCustomizeImageWithConfigFile(t.Context(), buildDir, configFile2, outImageFilePath, outImageFilePath2, "raw",
		baseImageInfo.PreviewFeatures)
	if !assert.NoError(t, err) {
		return
	}

	verifyUsrVerity(t, buildDir, outImageFilePath2, ukiFilesChecksums, nil)
}

func TestCustomizeImageVerityUsrUkiRecustomize(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinux3Plus {
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

	testTempDir := filepath.Join(tmpDir, fmt.Sprintf("TestCustomizeImageUsrVerityUkiRecustomize_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	configFile := filepath.Join(testDir, verityUsrUkiConfigFile(t, baseImageInfo))

	err = basicCustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, outImageFilePath, "raw",
		baseImageInfo.PreviewFeatures)
	if !assert.NoError(t, err) {
		return
	}

	ukiFilesChecksums, addonFilesChecksums, ok := verifyUsrVerity(t, buildDir, outImageFilePath, nil, nil)
	if !ok {
		return
	}

	// Re-customize the UKI image with 'os.uki.mode: create', os.kernelCommandLine.extraCommandLine, and
	// 'os.bootloader.resetType: hard-reset'.
	outImageFilePath2 := filepath.Join(testTempDir, "image2.raw")
	configFile2 := filepath.Join(testDir, "verity-reinit-usr-uki.yaml")

	err = basicCustomizeImageWithConfigFile(t.Context(), buildDir, configFile2, outImageFilePath, outImageFilePath2, "raw",
		baseImageInfo.PreviewFeatures)
	if !assert.NoError(t, err) {
		return
	}

	newUkiFilesChecksums, newAddonFilesChecksums, ok := verifyUsrVerity(t, buildDir, outImageFilePath2, nil, nil)
	if !ok {
		return
	}

	verifyUsrVerityUkiRecustomized(t, buildDir, outImageFilePath2,
		ukiFilesChecksums, newUkiFilesChecksums, addonFilesChecksums, newAddonFilesChecksums,
		"rd.info")
}

func TestCustomizeImageVerityUsrUkiPassthrough(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinux3Plus {
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

	testTempDir := filepath.Join(tmpDir, fmt.Sprintf("TestCustomizeImageUsrVerityUkiPassthrough_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	configFile := filepath.Join(testDir, verityUsrUkiConfigFile(t, baseImageInfo))

	err = basicCustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, outImageFilePath, "raw",
		baseImageInfo.PreviewFeatures)
	if !assert.NoError(t, err) {
		return
	}

	ukiFilesChecksums, _, ok := verifyUsrVerity(t, buildDir, outImageFilePath, nil, nil)
	if !ok {
		return
	}

	outImageFilePath2 := filepath.Join(testTempDir, "image-passthrough.raw")
	configFile2 := filepath.Join(testDir, "verity-usr-uki-passthrough.yaml")

	err = basicCustomizeImageWithConfigFile(t.Context(), buildDir, configFile2, outImageFilePath, outImageFilePath2, "raw",
		baseImageInfo.PreviewFeatures)
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
	for _, baseImageInfo := range baseImageAzureLinux3Plus {
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

	testTempDir := filepath.Join(tmpDir, fmt.Sprintf("TestCustomizeImageRootVerityUki_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	configFile := filepath.Join(testDir, verityRootUkiConfigFile(t, baseImageInfo))

	// Customize image.
	err = basicCustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, outImageFilePath, "raw",
		baseImageInfo.PreviewFeatures)
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

	err = basicCustomizeImageWithConfigFile(t.Context(), buildDir, configFile2, outImageFilePath, outImageFilePath2, "raw",
		baseImageInfo.PreviewFeatures)
	if !assert.NoError(t, err) {
		return
	}

	verifyRootVerityUki(t, buildDir, outImageFilePath2, nil)
}

func TestCustomizeImageVerityUsrUkiRecustomizeCmdline(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinux3Plus {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageVerityUsrUkiRecustomizeCmdlineHelper(t, baseImageInfo)
		})
	}
}

func testCustomizeImageVerityUsrUkiRecustomizeCmdlineHelper(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	ukifyExists, err := file.CommandExists("ukify")
	assert.NoError(t, err)
	if !ukifyExists {
		t.Skip("The 'ukify' command is not available")
	}

	testTempDir := filepath.Join(tmpDir, fmt.Sprintf("TestCustomizeImageVerityUsrUkiRecustomizeCmdline_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")

	// Pass 1: turn the grub-based base image into a UKI image. IC removes grub.cfg and writes
	// UKIs to the ESP, matching the ACL template image layout (UKIs present, no grub.cfg).
	outImageFilePath1 := filepath.Join(testTempDir, "uki-base.raw")
	configFile1 := filepath.Join(testDir, verityUsrUkiConfigFile(t, baseImageInfo))
	err = basicCustomizeImageWithConfigFile(t.Context(), buildDir, configFile1, baseImage, outImageFilePath1, "raw",
		baseImageInfo.PreviewFeatures)
	if !assert.NoError(t, err) {
		return
	}

	ukiFilesChecksums1, addonFilesChecksums1, ok := verifyUsrVerity(t, buildDir, outImageFilePath1, nil, nil)
	if !ok {
		return
	}

	// Pass 2: re-customize that UKI image with 'os.uki.mode: create' and os.kernelCommandLine.extraCommandLine but no
	// 'os.bootloader.resetType: hard-reset'.
	outImageFilePath2 := filepath.Join(testTempDir, "image.raw")
	configFile2 := filepath.Join(testDir, "verity-reinit-usr-uki-cmdline.yaml")
	err = basicCustomizeImageWithConfigFile(t.Context(), buildDir, configFile2, outImageFilePath1, outImageFilePath2, "raw",
		baseImageInfo.PreviewFeatures)
	if !assert.NoError(t, err) {
		return
	}

	ukiFilesChecksums2, addonFilesChecksums2, ok := verifyUsrVerity(t, buildDir, outImageFilePath2, nil, nil)
	if !ok {
		return
	}

	// Create mode must preserve each main UKI while regenerating the addon, and the gb200
	// extraCommandLine must have reached the regenerated UKIs — the specific behaviour this test
	// exercises via the default (non hard-reset) AddKernelCommandLine path.
	verifyUsrVerityUkiRecustomized(t, buildDir, outImageFilePath2,
		ukiFilesChecksums1, ukiFilesChecksums2, addonFilesChecksums1, addonFilesChecksums2,
		"console=ttyAMA0,115200n8")
}

// verifyUsrVerityUkiRecustomized checks that re-customizing a usr-verity UKI image in create mode
// preserved each main UKI (kernel/initramfs/os-release unchanged) while regenerating its addon
// (cmdline changed), and that /boot contains only the ESP. When expectedCmdlineArgs are given, it
// also asserts every regenerated UKI's kernel command-line contains each of them.
func verifyUsrVerityUkiRecustomized(t *testing.T, buildDir string, imagePath string,
	oldUkiChecksums, newUkiChecksums, oldAddonChecksums, newAddonChecksums map[string]string,
	expectedCmdlineArgs ...string,
) {
	// The main UKI (kernel + os-release + initramfs) must not change; only the addon (cmdline) does.
	for ukiFile, oldChecksum := range oldUkiChecksums {
		newChecksum, exists := newUkiChecksums[ukiFile]
		assert.True(t, exists, "UKI file should exist after re-customization: %s", ukiFile)
		assert.Equal(t, oldChecksum, newChecksum,
			"Main UKI checksum should NOT change (only cmdline in addon changes): %s", ukiFile)
	}

	// The addon (cmdline) must change.
	for addonFile, oldChecksum := range oldAddonChecksums {
		newChecksum, exists := newAddonChecksums[addonFile]
		assert.True(t, exists, "Addon file should exist after re-customization: %s", addonFile)
		assert.NotEqual(t, oldChecksum, newChecksum,
			"Addon checksum should change after re-customization: %s", addonFile)
	}

	// Mount parent-first (/, then /boot, then the ESP) so the ESP is actually exposed for reads.
	mountPoints := []testutils.MountPoint{
		{PartitionNum: 5, Path: "/", FileSystemType: "ext4"},
		{PartitionNum: 2, Path: "/boot", FileSystemType: "ext4"},
		{PartitionNum: 1, Path: "/boot/efi", FileSystemType: "vfat"},
	}
	imageConnection, err := testutils.ConnectToImage(buildDir, imagePath, false /*includeDefaultMounts*/, mountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// /boot should only contain "efi" and optionally "lost+found".
	bootPath := filepath.Join(imageConnection.Chroot().RootDir(), "/boot")
	bootEntries, err := os.ReadDir(bootPath)
	assert.NoError(t, err)
	assert.LessOrEqual(t, len(bootEntries), 2, "/boot should contain at most 2 entries: 'efi' and 'lost+found'")
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

	// Optionally verify specific kernel command-line args reached the regenerated UKIs.
	if len(expectedCmdlineArgs) > 0 {
		espPath := filepath.Join(imageConnection.Chroot().RootDir(), "/boot/efi")
		kernelToArgs, err := extractKernelCmdlineFromUkiEfis(espPath, buildDir)
		if !assert.NoError(t, err) {
			return
		}
		assert.GreaterOrEqual(t, len(kernelToArgs), 1, "expected at least one UKI in the ESP")
		for kernel, args := range kernelToArgs {
			for _, expected := range expectedCmdlineArgs {
				assert.Contains(t, args, expected,
					"regenerated UKI for kernel (%s) should contain cmdline arg (%s)", kernel, expected)
			}
		}
	}
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

// verityUsrInlineUkiConfigFile returns the verity-usr-inline-uki test config file appropriate for the
// given base image version (azl3 vs azl4) and host architecture.
func verityUsrInlineUkiConfigFile(t *testing.T, baseImageInfo testBaseImageInfo) string {
	switch baseImageInfo.Version {
	case baseImageVersionAzl3:
		return "verity-usr-inline-uki-azl3.yaml"
	case baseImageVersionAzl4:
		return fmt.Sprintf("verity-usr-inline-uki-%s-azl4.yaml", runtime.GOARCH)
	default:
		t.Fatalf("unsupported base image version for verity-usr-inline-uki test: %s", baseImageInfo.Version)
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

// TestCustomizeImageVerityUsrUkiRecustomizeNulPaddedCmdline is the end-to-end regression test for a failure where
// re-customizing a UKI usr-verity image aborted because its addon `.cmdline` PE section carried trailing NUL padding.
//
// The base-image validation reads the UKI addon `.cmdline`, discovers the usr-verity partition, and parses
// `systemd.verity_usr_options` (including the numeric `hash-offset` at the end), then re-provisions verity and rebuilds
// the UKI.
//
// A `.cmdline` section holds a NUL-terminated string. When a tool rewrites it with shorter content
// (for example `objcopy --update-section`), the section keeps its original size and the freed bytes are zero-filled,
// leaving trailing NUL padding after the last argument. If that padding is not stripped, the trailing `hash-offset
// value fails to parse and customization aborts with "cannot validate target OS of the base image".
//
// This test builds an inline-usr-verity UKI image (inline verity, where data and hash is on the same partition, makes
// IC append `...,hash-offset=<N>` to the addon command line), rewrites the addon `.cmdline` with shorter NUL-terminated
// content to recreate the padding, then re-customizes and asserts success.
func TestCustomizeImageVerityUsrUkiRecustomizeNulPaddedCmdline(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinux3Plus {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageVerityUsrUkiRecustomizeNulPaddedCmdlineHelper(t, baseImageInfo)
		})
	}
}

func testCustomizeImageVerityUsrUkiRecustomizeNulPaddedCmdlineHelper(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	for _, command := range []string{"ukify", "objcopy"} {
		exists, err := file.CommandExists(command)
		assert.NoError(t, err)
		if !exists {
			t.Skipf("The '%s' command is not available", command)
		}
	}

	testTempDir := filepath.Join(tmpDir, fmt.Sprintf("TestCustomizeImageVerityUsrUkiRecustomizeNulPaddedCmdline_%s",
		baseImageInfo.Name))
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")

	// Build an inline-usr-verity UKI image whose addon command line ends with
	// `systemd.verity_usr_options=...,hash-offset=<N>`.
	ukiImageFilePath := filepath.Join(testTempDir, "uki-verity.raw")
	configFile := filepath.Join(testDir, verityUsrInlineUkiConfigFile(t, baseImageInfo))
	err := basicCustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, ukiImageFilePath, "raw",
		baseImageInfo.PreviewFeatures)
	if !assert.NoError(t, err) {
		return
	}

	// Rewrite the addon `.cmdline` with shorter, NUL-terminated content so the re-read command line carries trailing
	// NUL padding after its last argument.
	if !injectAddonCmdlineNulPadding(t, buildDir, ukiImageFilePath) {
		return
	}

	// Re-customize.
	outImageFilePath := filepath.Join(testTempDir, "recustomized.raw")
	reinitConfigFile := filepath.Join(testDir, "verity-reinit-usr-uki.yaml")
	err = basicCustomizeImageWithConfigFile(t.Context(), buildDir, reinitConfigFile, ukiImageFilePath, outImageFilePath,
		"raw", baseImageInfo.PreviewFeatures)
	if !assert.NoError(t, err) {
		return
	}

	// Validate the re-customized image. Reading the UKI addon command line back is the code path the fix touches, so
	// this also confirms the usr-verity configuration is well-formed, including a parseable inline hash-offset.
	verifyUsrInlineVerityUki(t, buildDir, outImageFilePath)
}

// injectAddonCmdlineNulPadding rewrites every UKI addon `.cmdline` section on the image's ESP with shorter,
// NUL-terminated content using `objcopy --update-section`, so the re-read command line carries trailing NUL padding
// after its last argument.
//
// `objcopy --update-section` keeps the section's original size and zero-pads the freed bytes, which is how trailing
// NULs end up in a `.cmdline` section in practice. The content is shortened by dropping the throwaway
// nulPaddingFillerArg argument added by the build config, keeping the verity `hash-offset` value last so the padding
// lands on it.
//
// Returns false (after failing the test) if no addon carrying the command line was found to patch.
func injectAddonCmdlineNulPadding(t *testing.T, buildDir string, imageFilePath string) bool {
	imageConnection, err := testutils.ConnectToImage(buildDir, imageFilePath, false, /*includeDefaultMounts*/
		[]testutils.MountPoint{
			{
				PartitionNum:   1,
				Path:           "/boot/efi",
				FileSystemType: "vfat",
			},
		})
	if !assert.NoError(t, err) {
		return false
	}
	defer imageConnection.Close()

	espDir := filepath.Join(imageConnection.Chroot().RootDir(), "/boot/efi")
	addonFiles, err := getUkiAddonFiles(espDir)
	if !assert.NoError(t, err) {
		return false
	}
	if !assert.NotEmpty(t, addonFiles, "expected at least one UKI addon on the ESP") {
		return false
	}

	dumpPath := filepath.Join(buildDir, "addon-cmdline-orig.bin")
	shortPath := filepath.Join(buildDir, "addon-cmdline-short.bin")

	patched := false
	for _, addonFile := range addonFiles {
		_, _, err := shell.Execute("objcopy", "--dump-section", ".cmdline="+dumpPath, addonFile)
		if !assert.NoError(t, err) {
			return false
		}

		content, err := os.ReadFile(dumpPath)
		if !assert.NoError(t, err) {
			return false
		}

		// IC writes a clean command line (no NULs). Drop the throwaway nulPaddingFillerArg to make the content shorter
		// than the section, then re-terminate with a single NUL. objcopy keeps the original section size and zero-pads
		// the freed bytes, leaving trailing NULs after the last argument.
		cmdline := string(content)
		shortened := strings.Replace(cmdline, " "+nulPaddingFillerArg, "", 1)
		if shortened == cmdline {
			// Not the addon carrying the padded command line; leave it untouched.
			continue
		}

		err = os.WriteFile(shortPath, append([]byte(shortened), 0), 0o644)
		if !assert.NoError(t, err) {
			return false
		}

		_, _, err = shell.Execute("objcopy", "--update-section", ".cmdline="+shortPath, addonFile)
		if !assert.NoError(t, err) {
			return false
		}
		patched = true
	}

	return assert.True(t, patched, "no addon carrying the throwaway "+nulPaddingFillerArg+" argument was found to patch")
}

// verifyUsrInlineVerityUki validates that an inline-usr-verity UKI image is well-formed. It reads the usr-verity
// kernel command line back from the UKI addons on the ESP (the code path the NUL-trim fix touches), checks the
// systemd.verity_usr_* arguments including a parseable inline hash-offset, and runs `veritysetup verify` against the
// usr partition.
func verifyUsrInlineVerityUki(t *testing.T, buildDir string, imageFilePath string) {
	imageConnection, err := testutils.ConnectToImage(buildDir, imageFilePath, false, /*includeDefaultMounts*/
		[]testutils.MountPoint{
			{
				PartitionNum:   1,
				Path:           "/boot/efi",
				FileSystemType: "vfat",
			},
		})
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	partitions, err := getDiskPartitionsMap(imageConnection.Loopback().DevicePath())
	if !assert.NoError(t, err, "get disk partitions") {
		return
	}

	// Inline verity keeps the usr data and hash tree on the same partition, so the data and hash device and id are
	// both the usr partition.
	espPath := filepath.Join(imageConnection.Chroot().RootDir(), "/boot/efi")
	usrDevice := testutils.PartitionDevPath(imageConnection, 3)
	usrPartUuid := "PARTUUID=" + partitions[3].PartUuid
	verifyVerityUki(t, espPath, usrDevice, usrDevice, usrPartUuid, usrPartUuid, "usr", buildDir, "rd.info",
		"panic-on-corruption", true /*inlineVerity*/)

	err = imageConnection.CleanClose()
	assert.NoError(t, err)
}
