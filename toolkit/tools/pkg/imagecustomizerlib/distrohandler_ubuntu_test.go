// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/stretchr/testify/assert"
)

func TestUbuntuDistroHandlerValidateConfigAcceptsLegacyPreviewFeature(t *testing.T) {
	tests := []struct {
		name    string
		version string
		err     error
	}{
		{
			name:    "Ubuntu2204",
			version: "22.04",
			err:     ErrUbuntu2204PreviewFeatureRequired,
		},
		{
			name:    "Ubuntu2404",
			version: "24.04",
			err:     ErrUbuntu2404PreviewFeatureRequired,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := newUbuntuDistroHandler(test.version)
			config := &ResolvedConfig{
				PreviewFeatures: []imagecustomizerapi.PreviewFeature{imagecustomizerapi.PreviewFeatureUbuntu},
			}

			err := handler.ValidateConfig(config)
			assert.NoError(t, err)

			config.PreviewFeatures = nil
			err = handler.ValidateConfig(config)
			assert.ErrorIs(t, err, test.err)
		})
	}
}
