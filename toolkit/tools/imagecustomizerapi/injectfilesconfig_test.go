// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInjectFilesConfig_Valid(t *testing.T) {
	cfg := InjectFilesConfig{
		PreviewFeatures: []PreviewFeature{PreviewFeatureInjectFiles},
		InjectFiles: []InjectArtifactMetadata{
			{
				Source:      "./shim/bootx64.efi",
				Destination: "/EFI/BOOT/bootx64.efi",
				Type:        OutputArtifactsItemShim,
				Partition: InjectFilePartition{
					MountIdType: MountIdentifierTypePartUuid,
					Id:          "5678-EFGH",
				},
			},
		},
	}

	err := cfg.IsValid()
	assert.NoError(t, err)
}

func TestInjectFilesConfig_MissingPreviewFeature(t *testing.T) {
	cfg := InjectFilesConfig{
		PreviewFeatures: []PreviewFeature{},
		InjectFiles:     []InjectArtifactMetadata{},
	}

	err := cfg.IsValid()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "inject-files")
}

func TestInjectFilesConfig_InvalidInjectArtifact(t *testing.T) {
	cfg := InjectFilesConfig{
		PreviewFeatures: []PreviewFeature{PreviewFeatureInjectFiles},
		InjectFiles: []InjectArtifactMetadata{
			{
				Source:      "",
				Destination: "/EFI/BOOT/bootx64.efi",
				Type:        OutputArtifactsItemShim,
				Partition: InjectFilePartition{
					MountIdType: MountIdentifierTypePartUuid,
					Id:          "5678-EFGH",
				},
			},
		},
	}

	err := cfg.IsValid()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "injectFiles[0] is invalid")
}
