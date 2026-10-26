//go:build !acl_root_verity

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import "context"

func convertAclRootVerity(_ context.Context, _ *ResolvedConfig) error {
	return ErrAclRootVerityUnsupportedBuild
}
