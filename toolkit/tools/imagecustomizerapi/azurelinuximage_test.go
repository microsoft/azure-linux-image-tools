// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAzureLinuxImageIsValidOk(t *testing.T) {
	value := AzureLinuxImage{
		Variant: "minimal-os",
		Version: "3.0.20250910",
	}
	assert.NoError(t, value.IsValid())
}

func TestInputImageIsValidOciBadVariant(t *testing.T) {
	value := AzureLinuxImage{
		Variant: "_minimal-os",
		Version: "3.0.20250910",
	}
	err := value.IsValid()
	assert.ErrorContains(t, err, "invalid 'variant' field")
}

func TestInputImageIsValidOciBadVersion(t *testing.T) {
	value := AzureLinuxImage{
		Variant: "minimal-os",
		Version: "3",
	}
	err := value.IsValid()
	assert.ErrorContains(t, err, "invalid 'version' field")
}
