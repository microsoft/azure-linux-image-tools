// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/stretchr/testify/assert"
)

func TestVerityReinitializePreviewFeatureRequired(t *testing.T) {
	newRC := func(reinit imagecustomizerapi.ReinitializeVerityType) *ResolvedConfig {
		rc := &ResolvedConfig{}
		rc.Storage.ReinitializeVerity = reinit
		return rc
	}

	// No base verity -> never required, regardless of reinitializeVerity.
	assert.False(t, verityReinitializePreviewFeatureRequired(newRC(imagecustomizerapi.ReinitializeVerityTypeDefault), false))
	assert.False(t, verityReinitializePreviewFeatureRequired(newRC(imagecustomizerapi.ReinitializeVerityTypeAll), false))

	// Base has verity but not re-sealing (unset/none) -> not required.
	// This is the output.artifacts extraction / passthrough-UKI case: verity stays read-only.
	assert.False(t, verityReinitializePreviewFeatureRequired(newRC(imagecustomizerapi.ReinitializeVerityTypeDefault), true))
	assert.False(t, verityReinitializePreviewFeatureRequired(newRC(imagecustomizerapi.ReinitializeVerityTypeNone), true))

	// Base has verity AND re-sealing requested -> feature required.
	assert.True(t, verityReinitializePreviewFeatureRequired(newRC(imagecustomizerapi.ReinitializeVerityTypeAll), true))
}
