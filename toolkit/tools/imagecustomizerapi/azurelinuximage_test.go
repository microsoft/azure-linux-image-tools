// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAzureLinuxImageIsValidAzl3Ok(t *testing.T) {
	value := AzureLinuxImage{
		Variant: "minimal-os",
		Version: "3.0.20250910",
	}
	assert.NoError(t, value.IsValid())
}

func TestAzureLinuxImageIsValidAzl4LatestOk(t *testing.T) {
	value := AzureLinuxImage{
		Variant: "minimal-os",
		Version: "4",
	}
	assert.NoError(t, value.IsValid())
}

func TestAzureLinuxImageIsValidAzl4LatestDateOk(t *testing.T) {
	value := AzureLinuxImage{
		Variant: "minimal-os",
		Version: "4-2026102701",
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

func TestInputImageIsValidOciBadVersionAzl3(t *testing.T) {
	value := AzureLinuxImage{
		Variant: "minimal-os",
		Version: "3",
	}
	err := value.IsValid()
	assert.ErrorContains(t, err, "invalid 'version' field:\n")
	assert.ErrorContains(t, err, "the version format <MAJOR>[-<DATE>] is only valid for AZL4+ (version='3')")
}

func TestInputImageIsValidOciBadVersionAzl4(t *testing.T) {
	value := AzureLinuxImage{
		Variant: "minimal-os",
		Version: "4.0",
	}
	err := value.IsValid()
	assert.ErrorContains(t, err, "invalid 'version' field")
	assert.ErrorContains(t, err, "the version format <MAJOR>.<MINOR>[.<DATE>] is only valid for AZL2 and AZL3 (version='4.0')")
}
