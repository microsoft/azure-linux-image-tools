// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package spdxmanifest

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildCreator(t *testing.T) {
	for _, test := range []struct {
		name        string
		toolVersion string
		creator     string
	}{
		{name: "development", toolVersion: "dev", creator: "Tool: imagecustomizer-dev"},
		{name: "empty", creator: "Tool: imagecustomizer-"},
		{name: "release", toolVersion: "1.7.0", creator: "Tool: imagecustomizer-1.7.0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			options := BuildOptions{
				Name:        "azurelinux",
				VersionInfo: "1.0",
				ToolVersion: test.toolVersion,
				Created:     "2025-01-01T00:00:00Z",
			}
			manifest, err := Build(options, nil)
			require.NoError(t, err)
			var document struct {
				CreationInfo struct {
					Creators []string `json:"creators"`
				} `json:"creationInfo"`
			}
			require.NoError(t, json.Unmarshal(manifest, &document))
			assert.Equal(t, []string{test.creator}, document.CreationInfo.Creators)
		})
	}
}

func TestDocumentNamespaceSeed(t *testing.T) {
	packages := []Package{{ID: "bash-5.2-1.azl3.x86_64"}}
	expectedUUID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("image/1.0/bash-5.2-1.azl3.x86_64"))
	assert.Equal(t, documentNamespaceBase+"/image-"+expectedUUID.String(), documentNamespace("image", "1.0", packages))
}
