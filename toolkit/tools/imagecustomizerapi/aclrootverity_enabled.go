//go:build acl_root_verity

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
	"slices"
)

const AclRootVerityVersionSuffix = " (build features: [acl-root-verity])"

func RequireAclRootVerity(previewFeatures []PreviewFeature) error {
	if !slices.Contains(previewFeatures, PreviewFeatureAclRootVerity) {
		return fmt.Errorf("preview feature (%s) must be enabled", PreviewFeatureAclRootVerity)
	}
	return nil
}
