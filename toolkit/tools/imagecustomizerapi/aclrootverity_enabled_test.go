//go:build acl_root_verity

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAclRootVerityBuildEnabled(t *testing.T) {
	assert.Equal(t, []PreviewFeature{PreviewFeatureAclRootVerity}, BuildFeatures())
	assert.NoError(t, PreviewFeatureAclRootVerity.IsValid())
	assert.NoError(t, (&Config{PreviewFeatures: []PreviewFeature{PreviewFeatureAclRootVerity}}).IsValid())
}

func TestRequireAclRootVerityEnabled(t *testing.T) {
	assert.ErrorContains(t, RequireAclRootVerity(nil), "must be enabled")
	assert.ErrorContains(t, RequireAclRootVerity([]PreviewFeature{PreviewFeatureUki}), "must be enabled")
	assert.NoError(t, RequireAclRootVerity([]PreviewFeature{PreviewFeatureUki, PreviewFeatureAclRootVerity}))
}
