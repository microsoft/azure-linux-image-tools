// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/stretchr/testify/assert"
)

func TestValidateSnapshotTimeInput(t *testing.T) {
	// Test both features - should fail as they are incompatible
	previewFeatures := []imagecustomizerapi.PreviewFeature{
		imagecustomizerapi.PreviewFeaturePackageSnapshotTime,
		imagecustomizerapi.PreviewFeatureFedora42,
	}

	err := validateSnapshotTimeInput("2023-10-10T10:10:10Z", previewFeatures)
	assert.ErrorContains(t, err, fmt.Sprintf("'%s' feature is not supported with '%s' feature",
		imagecustomizerapi.PreviewFeaturePackageSnapshotTime, imagecustomizerapi.PreviewFeatureFedora42))

	// Test with no preview features enabled
	err = validateSnapshotTimeInput("2023-10-10T10:10:10Z", []imagecustomizerapi.PreviewFeature{})
	assert.ErrorIs(t, err, ErrPackageSnapshotPreviewRequired)

	// Test with only package-snapshot-time feature - should succeed
	err = validateSnapshotTimeInput("2023-10-10T10:10:10Z", []imagecustomizerapi.PreviewFeature{imagecustomizerapi.PreviewFeaturePackageSnapshotTime})
	assert.NoError(t, err)

	// Test with only fedora-42 feature - should fail with preview required error
	err = validateSnapshotTimeInput("2023-10-10T10:10:10Z", []imagecustomizerapi.PreviewFeature{imagecustomizerapi.PreviewFeatureFedora42})
	assert.ErrorIs(t, err, ErrPackageSnapshotPreviewRequired)
}
