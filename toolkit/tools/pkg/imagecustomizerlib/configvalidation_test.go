// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/stretchr/testify/assert"
)

func TestValidateSnapshotTimeInput(t *testing.T) {
	// Test both features - should fail as they are incompatible
	config := &imagecustomizerapi.Config{
		PreviewFeatures: []imagecustomizerapi.PreviewFeature{
			imagecustomizerapi.PreviewFeaturePackageSnapshotTime,
			imagecustomizerapi.PreviewFeatureFedora42,
		},
	}

	_, err := validatePackageSnapshotTime(imagecustomizerapi.PackageSnapshotTime("2023-10-10T10:10:10Z"), config)
	assert.ErrorIs(t, err, ErrUnsupportedFedoraFeature)

	// Test with no preview features enabled
	config.PreviewFeatures = nil

	_, err = validatePackageSnapshotTime("2023-10-10T10:10:10Z", config)
	assert.ErrorIs(t, err, ErrPackageSnapshotPreviewRequired)

	// Test with only package-snapshot-time feature - should succeed
	config.PreviewFeatures = []imagecustomizerapi.PreviewFeature{imagecustomizerapi.PreviewFeaturePackageSnapshotTime}

	_, err = validatePackageSnapshotTime("2023-10-10T10:10:10Z", config)
	assert.NoError(t, err)

	// Test with only fedora-42 feature - should fail with preview required error
	config.PreviewFeatures = []imagecustomizerapi.PreviewFeature{imagecustomizerapi.PreviewFeatureFedora42}

	_, err = validatePackageSnapshotTime("2023-10-10T10:10:10Z", config)
	assert.ErrorIs(t, err, ErrPackageSnapshotPreviewRequired)
}
