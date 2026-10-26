// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
	"slices"
)

// BuildFeatures lists capabilities requiring a special build.
func BuildFeatures() []PreviewFeature {
	if aclRootVerityCompiled {
		return []PreviewFeature{PreviewFeatureAclRootVerity}
	}
	return nil
}

// RequireAclRootVerity requires both build-time support and explicit runtime opt-in.
func RequireAclRootVerity(previewFeatures []PreviewFeature) error {
	if !aclRootVerityCompiled {
		return fmt.Errorf("preview feature (%s) requires the acl_root_verity Go build tag", PreviewFeatureAclRootVerity)
	}
	if !slices.Contains(previewFeatures, PreviewFeatureAclRootVerity) {
		return fmt.Errorf("preview feature (%s) must be enabled", PreviewFeatureAclRootVerity)
	}
	return nil
}
