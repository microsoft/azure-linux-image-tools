// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateResourceTypesContains_MatchingType_Pass(t *testing.T) {
	types := ValidateResourceTypes{ValidateResourceTypeFiles}
	result := types.Contains(ValidateResourceTypeFiles)
	assert.True(t, result)
}

func TestValidateResourceTypesContains_NonMatchingType_Fail(t *testing.T) {
	types := ValidateResourceTypes{ValidateResourceTypeFiles}
	result := types.Contains(ValidateResourceTypeOci)
	assert.False(t, result)
}

func TestValidateResourceTypesContains_AllTypeWithSupported_Pass(t *testing.T) {
	types := ValidateResourceTypes{ValidateResourceTypeAll}
	for _, rt := range SupportedValidateResourceTypes() {
		result := types.Contains(ValidateResourceType(rt))
		assert.True(t, result)
	}
}

func TestValidateResourceTypesContains_EmptyList_Fail(t *testing.T) {
	types := ValidateResourceTypes{}
	result := types.Contains(ValidateResourceTypeFiles)
	assert.False(t, result)
}

func TestValidateResourceTypesValidateFiles_FilesPresent_Pass(t *testing.T) {
	types := ValidateResourceTypes{ValidateResourceTypeFiles}
	result := types.ValidateFiles()
	assert.True(t, result)
}

func TestValidateResourceTypesValidateFiles_FilesAbsent_Fail(t *testing.T) {
	types := ValidateResourceTypes{ValidateResourceTypeOci}
	result := types.ValidateFiles()
	assert.False(t, result)
}

func TestValidateResourceTypesValidateOci_OciPresent_Pass(t *testing.T) {
	types := ValidateResourceTypes{ValidateResourceTypeOci}
	result := types.ValidateOci()
	assert.True(t, result)
}

func TestValidateResourceTypesValidateOci_OciAbsent_Fail(t *testing.T) {
	types := ValidateResourceTypes{ValidateResourceTypeFiles}
	result := types.ValidateOci()
	assert.False(t, result)
}
