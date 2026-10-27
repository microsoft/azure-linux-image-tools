//go:build acl_root_verity

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
)

// Prepare only the source/layout; customization and finalization stay in the shared pipeline.
func prepareAclRootVerity(_ context.Context, rc *ResolvedConfig) error {
	if rc == nil {
		return fmt.Errorf("%w: resolved configuration is required", ErrInvalidParameters)
	}
	if err := imagecustomizerapi.RequireAclRootVerity(rc.PreviewFeatures); err != nil {
		return fmt.Errorf("%w:\n%w", ErrInvalidParameters, err)
	}
	return ErrAclRootVerityNotImplemented
}
