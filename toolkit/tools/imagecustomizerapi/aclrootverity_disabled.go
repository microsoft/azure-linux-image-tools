//go:build !acl_root_verity

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import "fmt"

const AclRootVerityVersionSuffix = ""

func RequireAclRootVerity(_ []PreviewFeature) error {
	return fmt.Errorf("preview feature (%s) requires the acl_root_verity Go build tag", PreviewFeatureAclRootVerity)
}
