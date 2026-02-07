// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/stretchr/testify/assert"
)

func TestValidateConfigOptionsIsValid_EmptyOptions_Pass(t *testing.T) {
	options := ValidateConfigOptions{}
	err := options.IsValid()
	assert.NoError(t, err)
}

func TestValidateConfigOptionsIsValid_ValidResourceTypeFiles_Pass(t *testing.T) {
	options := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{
			imagecustomizerapi.ValidateResourceTypeFiles,
		},
	}
	err := options.IsValid()
	assert.NoError(t, err)
}

func TestValidateConfigOptionsIsValid_ValidResourceTypeOciWithBuildDir_Pass(t *testing.T) {
	options := ValidateConfigOptions{
		BuildDir: "/tmp/build",
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{
			imagecustomizerapi.ValidateResourceTypeOci,
		},
	}
	err := options.IsValid()
	assert.NoError(t, err)
}

func TestValidateConfigOptionsIsValid_ValidResourceTypeAllWithBuildDir_Pass(t *testing.T) {
	options := ValidateConfigOptions{
		BuildDir: "/tmp/build",
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{
			imagecustomizerapi.ValidateResourceTypeAll,
		},
	}
	err := options.IsValid()
	assert.NoError(t, err)
}

func TestValidateConfigOptionsIsValid_InvalidResourceType_Fail(t *testing.T) {
	options := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{
			imagecustomizerapi.ValidateResourceType("invalid"),
		},
	}
	err := options.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid validate resource type")
}

func TestValidateConfigOptionsIsValid_OciWithoutBuildDir_Fail(t *testing.T) {
	options := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{
			imagecustomizerapi.ValidateResourceTypeOci,
		},
	}
	err := options.IsValid()
	assert.ErrorIs(t, err, ErrValidateConfigOptionsBuildDirRequiredForOci)
}

func TestValidateConfigOptionsIsValid_AllWithoutBuildDir_Fail(t *testing.T) {
	options := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{
			imagecustomizerapi.ValidateResourceTypeAll,
		},
	}
	err := options.IsValid()
	assert.ErrorIs(t, err, ErrValidateConfigOptionsBuildDirRequiredForOci)
}
