// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSignArtifactsConfigIsValidEphemeral(t *testing.T) {
	config := SignArtifactsConfig{
		SigningMethod: SigningMethod{
			Ephemeral: &SigningMethodEphemeral{},
		},
	}
	err := config.IsValid()
	assert.NoError(t, err)
}

func TestSignArtifactsConfigIsValidNoMethod(t *testing.T) {
	config := SignArtifactsConfig{}
	err := config.IsValid()
	assert.ErrorContains(t, err, "invalid 'signingMethod' field:")
	assert.ErrorContains(t, err, "one and only one signing method must be specified")
}
