//go:build acl_root_verity

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
)

func convertAclRootVerity(_ context.Context, rc *ResolvedConfig) (imageMetadata, error) {
	if rc == nil {
		return imageMetadata{}, fmt.Errorf("%w: resolved configuration is required", ErrInvalidParameters)
	}
	if err := imagecustomizerapi.RequireAclRootVerity(rc.PreviewFeatures); err != nil {
		return imageMetadata{}, fmt.Errorf("%w:\n%w", ErrInvalidParameters, err)
	}
	return imageMetadata{}, ErrAclRootVerityNotImplemented
}
