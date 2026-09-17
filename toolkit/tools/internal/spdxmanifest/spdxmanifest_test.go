// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package spdxmanifest

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/sirupsen/logrus"
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

func TestBuildPreservesPackageMetadata(t *testing.T) {
	manifest, err := Build(BuildOptions{
		Name:    "image",
		Created: "2025-01-01T00:00:00Z",
	}, []Package{
		{ID: "first", Name: "example", Vendor: `Company "Research" \ <&>`},
		{ID: "second", Name: "vendorless"},
	})
	require.NoError(t, err)

	var document struct {
		Packages []map[string]any `json:"packages"`
	}
	require.NoError(t, json.Unmarshal(manifest, &document))
	require.Len(t, document.Packages, 3)
	assert.NotContains(t, document.Packages[0], "versionInfo")
	assert.NotContains(t, document.Packages[1], "versionInfo")
	assert.Equal(t, `Organization: Company "Research" \ <&>`, document.Packages[1]["supplier"])
	assert.Equal(t, "NOASSERTION", document.Packages[2]["supplier"])
	assert.NotContains(t, string(manifest), `\u0026`)
	assert.NotContains(t, string(manifest), `\u003c`)
	assert.NotContains(t, string(manifest), `\u003e`)
}

func TestBuildWarnsForLiteralNoAssertionVendor(t *testing.T) {
	originalLogger := logger.Log
	logger.Log = logrus.New()
	t.Cleanup(func() { logger.Log = originalLogger })
	output := &bytes.Buffer{}
	logger.Log.SetOutput(output)

	packages := []Package{
		{ID: "sentinel", Name: "sentinel", Vendor: "NOASSERTION"},
	}
	manifest, err := Build(BuildOptions{Name: "image"}, packages)
	require.NoError(t, err)

	var document struct {
		Packages []map[string]any `json:"packages"`
	}
	require.NoError(t, json.Unmarshal(manifest, &document))
	require.Len(t, document.Packages, 2)
	assert.Equal(t, "sentinel", document.Packages[1]["name"])
	assert.Equal(t, "NOASSERTION", document.Packages[1]["supplier"])
	assert.Equal(t, "NOASSERTION", packages[0].Vendor)
	assert.Contains(t, output.String(), "level=warning")
	assert.Contains(t, output.String(), "Clearing vendor (NOASSERTION) for package (sentinel)")
}
