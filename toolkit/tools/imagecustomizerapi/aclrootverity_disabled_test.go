//go:build !acl_root_verity

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAclRootVerityBuildDisabled(t *testing.T) {
	assert.Empty(t, AclRootVerityVersionSuffix)
	assert.ErrorContains(t, PreviewFeatureAclRootVerity.IsValid(), "requires the acl_root_verity Go build tag")
	assert.ErrorContains(t, (&Config{PreviewFeatures: []PreviewFeature{PreviewFeatureAclRootVerity}}).IsValid(),
		"requires the acl_root_verity Go build tag")
}

func TestRequireAclRootVerityDisabled(t *testing.T) {
	assert.ErrorContains(t, RequireAclRootVerity(nil), "requires the acl_root_verity Go build tag")
	assert.ErrorContains(t, RequireAclRootVerity([]PreviewFeature{PreviewFeatureAclRootVerity}),
		"requires the acl_root_verity Go build tag")
}
