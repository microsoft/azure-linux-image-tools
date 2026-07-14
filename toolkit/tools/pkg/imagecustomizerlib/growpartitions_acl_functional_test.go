// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	}

	err := basicCustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, outImageFilePath, "raw",
		previewFeatures)
	if !assert.NoError(t, err) {
		return
	}

	const targetUsrSize = uint64(2 * 1024 * 1024 * 1024)

	// Both USR partitions must have grown to (at least) the requested size, preserving the A/B pair.
	verifyAclPartitionSizes(t, outImageFilePath, map[string]uint64{
		aclPartLabelUsrA: targetUsrSize,
		aclPartLabelUsrB: targetUsrSize,
	})

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

func verifyAclPartitionSizes(t *testing.T, imageFile string, expectedMinSizes map[string]uint64) {
	loopback, err := safeloopback.NewLoopback(imageFile)
	require.NoError(t, err)
	defer loopback.Close()

	partitions, err := diskutils.GetDiskPartitions(loopback.DevicePath())
	require.NoError(t, err)

	byLabel := partitionsByLabel(partitions)
	for label, minSize := range expectedMinSizes {
		part, ok := byLabel[label]
		if !assert.Truef(t, ok, "partition %s not found", label) {
			continue
		}
		assert.GreaterOrEqualf(t, part.SizeInBytes, minSize,
			"partition %s size %d should be >= %d", label, part.SizeInBytes, minSize)
	}

	err = loopback.CleanClose()
	assert.NoError(t, err)
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
