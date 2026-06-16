// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	"github.com/stretchr/testify/assert"
)

// ACL must accept configs that request package operations now that an
// auto-provisioned tools chroot supports tdnf --installroot.
func TestAclValidateConfigAcceptsPackageOps(t *testing.T) {
	handler := newAclDistroHandler(targetos.TargetOsAzureContainerLinux3)

	rc := &ResolvedConfig{
		PreviewFeatures: []imagecustomizerapi.PreviewFeature{
			imagecustomizerapi.PreviewFeatureAzureContainerLinux,
		},
		ConfigChain: []*ConfigWithBasePath{
			{
				Config: &imagecustomizerapi.Config{
					OS: &imagecustomizerapi.OS{
						Packages: imagecustomizerapi.Packages{
							Install: []string{"vim"},
							Remove:  []string{"nano"},
							Update:  []string{"bash"},
						},
					},
				},
			},
		},
	}

	assert.NoError(t, handler.ValidateConfig(rc))
	assert.True(t, handler.NeedsToolsChroot())
}
