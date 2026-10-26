// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertAclRootVerity(t *testing.T) {
	tests := []struct {
		name        string
		config      *ResolvedConfig
		expectedErr error
	}{
		{"nil config", nil, ErrInvalidParameters},
		{"no opt-in", &ResolvedConfig{}, ErrInvalidParameters},
		{"unrelated opt-in", &ResolvedConfig{
			PreviewFeatures: []imagecustomizerapi.PreviewFeature{imagecustomizerapi.PreviewFeatureUki},
		}, ErrInvalidParameters},
		{"opted in", &ResolvedConfig{
			PreviewFeatures: []imagecustomizerapi.PreviewFeature{imagecustomizerapi.PreviewFeatureAclRootVerity},
		}, ErrAclRootVerityNotImplemented},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectedErr := tt.expectedErr
			if imagecustomizerapi.PreviewFeatureAclRootVerity.IsValid() != nil {
				expectedErr = ErrAclRootVerityUnsupportedBuild
			}
			assert.ErrorIs(t, convertAclRootVerity(t.Context(), tt.config), expectedErr)
		})
	}
}

func TestConvertAclRootVerityDoesNotModifyImage(t *testing.T) {
	directory := t.TempDir()
	image := filepath.Join(directory, "input.raw")
	contents := []byte("unchanged input image")
	require.NoError(t, os.WriteFile(image, contents, 0o600))

	rc := &ResolvedConfig{
		RawImageFile:    image,
		BuildDirAbs:     filepath.Join(directory, "build"),
		PreviewFeatures: []imagecustomizerapi.PreviewFeature{imagecustomizerapi.PreviewFeatureAclRootVerity},
	}
	before := *rc
	before.PreviewFeatures = slices.Clone(rc.PreviewFeatures)

	require.Error(t, convertAclRootVerity(t.Context(), rc))

	actual, err := os.ReadFile(image)
	require.NoError(t, err)
	assert.Equal(t, contents, actual)
	assert.Equal(t, before, *rc)
	_, err = os.Stat(rc.BuildDirAbs)
	assert.ErrorIs(t, err, os.ErrNotExist)
}
