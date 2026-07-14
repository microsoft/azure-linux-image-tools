// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// TestCustomizeImageAclGrowUsr grows ACL's /usr to 2 GiB, installs packages, and verifies the
// output has both USR partitions grown and that /usr verity still validates (mounts through the
// verity device). Requires an ACL base image (--base-image-core-acl); skipped otherwise.
func TestCustomizeImageAclGrowUsr(t *testing.T) {
	baseImageInfo := testBaseImageAclCore
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageAclGrowUsr")
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	configFile := filepath.Join(testDir, "acl-grow-usr-config.yaml")

	previewFeatures := []imagecustomizerapi.PreviewFeature{
		imagecustomizerapi.PreviewFeatureDistroVersion,
		imagecustomizerapi.PreviewFeatureAclGrowPartitions,
		imagecustomizerapi.PreviewFeatureReinitializeVerity,
		imagecustomizerapi.PreviewFeatureUki,
	}

	// Capture the base layout so we can assert ROOT is preserved and the disk grew by the delta.
	baseDiskSize, baseSizes := readAclLayoutForTest(t, baseImage)

	err := basicCustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, outImageFilePath, "raw",
		previewFeatures)
	if !assert.NoError(t, err) {
		return
	}

	const targetUsrSize = uint64(2 * 1024 * 1024 * 1024)

	// Both USR partitions must have grown to (at least) the requested size, preserving the A/B pair.
	grownDiskSize, grownSizes := readAclLayoutForTest(t, outImageFilePath)
	assert.GreaterOrEqual(t, grownSizes[aclPartLabelUsrA], targetUsrSize, "USR-A should be >= 2 GiB")
	assert.GreaterOrEqual(t, grownSizes[aclPartLabelUsrB], targetUsrSize, "USR-B should be >= 2 GiB")

	// The trailing ROOT partition must keep its original size (merely shifted, never shrunk).
	assert.Equal(t, baseSizes[aclPartLabelRoot], grownSizes[aclPartLabelRoot],
		"ROOT should keep its original size")

	// ESP and OEM are unchanged.
	assert.Equal(t, baseSizes[aclPartLabelEsp], grownSizes[aclPartLabelEsp], "ESP should be unchanged")
	assert.Equal(t, baseSizes[aclPartLabelOem], grownSizes[aclPartLabelOem], "OEM should be unchanged")

	// The overall disk must grow by exactly the USR growth delta (2 x (2 GiB - 1 GiB) = 2 GiB),
	// allowing for MiB rounding.
	expectedDelta := (grownSizes[aclPartLabelUsrA] - baseSizes[aclPartLabelUsrA]) +
		(grownSizes[aclPartLabelUsrB] - baseSizes[aclPartLabelUsrB])
	assert.InDelta(t, float64(baseDiskSize+expectedDelta), float64(grownDiskSize), float64(1024*1024),
		"disk should grow by exactly the partition growth delta")

	// The extra kernel cmdline args (uki: mode: create) must be baked into the regenerated UKIs,
	// even though ACL has no grub.cfg.
	verifyAclUkiCmdline(t, buildDir, outImageFilePath, []string{"flatcar.autologin", "console=ttyAMA0,115200n8"})

	// The regenerated initramfs must be zstd-compressed and include the systemd-veritysetup module;
	// otherwise /dev/mapper/usr never comes up and the image drops to an emergency shell.
	verifyAclInitramfsHasVerity(t, buildDir, outImageFilePath)

	// Verify /usr verity still validates: connecting with read-only verity mounts /usr through the
	// verity device, which fails if the re-seal / hash-offset is wrong.
	verifyAclUsrVerity(t, buildDir, outImageFilePath)
}

// TestCustomizeImageAclGrowRejectedForNonAcl ensures the 'acl' API is rejected for non-ACL images.
func TestCustomizeImageAclGrowRejectedForNonAcl(t *testing.T) {
	baseImage, baseImageInfo := checkSkipForCustomizeDefaultAzureLinuxImage(t)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageAclGrowRejectedForNonAcl")
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")

	config := &imagecustomizerapi.Config{
		Storage: imagecustomizerapi.Storage{
			ReinitializeVerity: imagecustomizerapi.ReinitializeVerityTypeAll,
		},
		Acl: &imagecustomizerapi.Acl{
			Usr: &imagecustomizerapi.AclPartitionGrow{Size: 2 * 1024 * 1024 * 1024},
		},
	}

	previewFeatures := append([]imagecustomizerapi.PreviewFeature{
		imagecustomizerapi.PreviewFeatureAclGrowPartitions,
		imagecustomizerapi.PreviewFeatureReinitializeVerity,
	}, baseImageInfo.PreviewFeatures...)

	err := basicCustomizeImage(t.Context(), buildDir, buildDir, config, baseImage, outImageFilePath, "raw",
		previewFeatures)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "only supported for Azure Container Linux")
}

// readAclLayoutForTest returns the image's virtual disk size and a map of ACL standard partition
// label -> size in bytes. The image is converted to raw for inspection if it is not already raw.
func readAclLayoutForTest(t *testing.T, imageFile string) (uint64, map[string]uint64) {
	rawFile := imageFile
	if ext := strings.ToLower(filepath.Ext(imageFile)); ext != ".raw" && ext != ".img" {
		rawFile = filepath.Join(t.TempDir(), "inspect-"+filepath.Base(imageFile)+".raw")
		err := shell.ExecuteLive(true /*squashErrors*/, "qemu-img", "convert", "-O", "raw", imageFile, rawFile)
		require.NoError(t, err, "converting %s to raw for inspection", imageFile)
	}

	stat, err := os.Stat(rawFile)
	require.NoError(t, err)
	diskSize := uint64(stat.Size())

	loopback, err := safeloopback.NewLoopback(rawFile)
	require.NoError(t, err)
	defer loopback.Close()

	partitions, err := diskutils.GetDiskPartitions(loopback.DevicePath())
	require.NoError(t, err)

	sizes := make(map[string]uint64)
	for label, part := range partitionsByLabel(partitions) {
		sizes[label] = part.SizeInBytes
	}

	err = loopback.CleanClose()
	assert.NoError(t, err)

	return diskSize, sizes
}

func verifyAclUsrVerity(t *testing.T, buildDir string, imageFile string) {
	imageConnection, _, verityMetadata, _, _, err := connectToExistingImage(t.Context(), imageFile, buildDir,
		"imageroot-verity", false /*includeDefaultMounts*/, true, /*readonly*/
		true /*readOnlyVerity*/, false /*ignoreOverlays*/, nil /*distroHandler*/)
	if !assert.NoError(t, err, "connecting with read-only verity should succeed if /usr is sealed correctly") {
		return
	}
	defer imageConnection.Close()

	// There should be a /usr verity device.
	hasUsr := false
	for _, m := range verityMetadata {
		if m.name == imagecustomizerapi.VerityUsrDeviceName {
			hasUsr = true
		}
	}
	assert.True(t, hasUsr, "expected a /usr verity device")

	err = imageConnection.CleanClose()
	assert.NoError(t, err)
}

// verifyAclUkiCmdline asserts that the regenerated UKIs on the output image's ESP carry the given
// kernel cmdline args (baked in via uki: mode: create, despite ACL having no grub.cfg).
func verifyAclUkiCmdline(t *testing.T, buildDir string, imageFile string, expectedArgs []string) {
	loopback, err := safeloopback.NewLoopback(imageFile)
	require.NoError(t, err)
	defer loopback.Close()

	partitions, err := diskutils.GetDiskPartitions(loopback.DevicePath())
	require.NoError(t, err)

	espPart, ok := partitionsByLabel(partitions)[aclPartLabelEsp]
	require.True(t, ok, "ESP partition not found")

	args, err := extractKernelCmdlineFromUki(&espPart, buildDir)
	require.NoError(t, err)

	argSet := make(map[string]bool)
	for _, a := range args {
		argSet[a.Arg] = true
	}
	for _, expected := range expectedArgs {
		assert.Truef(t, argSet[expected], "expected UKI cmdline to contain %q; got %v", expected, args)
	}

	err = loopback.CleanClose()
	assert.NoError(t, err)
}

// verifyAclInitramfsHasVerity asserts that each regenerated UKI's embedded initramfs is
// zstd-compressed and contains the systemd-veritysetup module (required to bring up the /usr
// dm-verity device at boot).
func verifyAclInitramfsHasVerity(t *testing.T, buildDir string, imageFile string) {
	loopback, err := safeloopback.NewLoopback(imageFile)
	require.NoError(t, err)
	defer loopback.Close()

	partitions, err := diskutils.GetDiskPartitions(loopback.DevicePath())
	require.NoError(t, err)

	espPart, ok := partitionsByLabel(partitions)[aclPartLabelEsp]
	require.True(t, ok, "ESP partition not found")

	espMountDir := filepath.Join(t.TempDir(), "esp")
	espMount, err := safemount.NewMount(espPart.Path, espMountDir, espPart.FileSystemType, unix.MS_RDONLY, "", true)
	require.NoError(t, err)
	defer espMount.Close()

	ukiFiles, err := getUkiFiles(espMountDir)
	require.NoError(t, err)
	require.NotEmpty(t, ukiFiles, "expected at least one UKI on the ESP")

	_, lsinitrdErr := exec.LookPath("lsinitrd")
	lsinitrdAvailable := lsinitrdErr == nil

	for _, uki := range ukiFiles {
		initrdPath := filepath.Join(t.TempDir(), "initrd.img")
		err := extractSectionFromUkiWithObjcopy(uki, ".initrd", initrdPath, buildDir)
		require.NoError(t, err, "extract .initrd from UKI (%s)", uki)

		assertInitramfsIsZstd(t, initrdPath)

		if lsinitrdAvailable {
			stdout, _, err := shell.Execute("lsinitrd", initrdPath)
			require.NoError(t, err, "lsinitrd on %s", initrdPath)
			assert.Contains(t, stdout, "systemd-veritysetup",
				"regenerated initramfs (%s) is missing the systemd-veritysetup module", uki)
		} else {
			t.Log("lsinitrd not available; skipping systemd-veritysetup module content check")
		}
	}

	err = espMount.CleanClose()
	assert.NoError(t, err)
}

// assertInitramfsIsZstd fails if the initramfs is gzip-compressed (the symptom of a broken regen)
// and passes when it is zstd. If the leading bytes are neither (e.g. an uncompressed early-CPIO
// microcode image precedes the main archive), the check is skipped since compression can't be
// determined from the header alone.
func assertInitramfsIsZstd(t *testing.T, initrdPath string) {
	f, err := os.Open(initrdPath)
	require.NoError(t, err)
	defer f.Close()

	magic := make([]byte, 4)
	_, err = f.Read(magic)
	require.NoError(t, err)

	gzipMagic := []byte{0x1f, 0x8b}
	zstdMagic := []byte{0x28, 0xb5, 0x2f, 0xfd}

	switch {
	case string(magic[:2]) == string(gzipMagic):
		t.Errorf("regenerated initramfs is gzip-compressed; expected zstd (ACL mandates compress=zstd)")
	case string(magic) == string(zstdMagic):
		// Expected.
	default:
		t.Logf("initramfs compression could not be determined from header (%x); skipping zstd check", magic)
	}
}
