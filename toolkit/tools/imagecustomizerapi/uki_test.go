// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestUkiIsValid(t *testing.T) {
	validUki := Uki{
		Kernels: UkiKernels{
			Auto: false,
			Kernels: []string{
				"6.6.51.1-5.azl3",
				"5.10.120-4.custom",
			},
		},
	}

	err := validUki.IsValid()
	assert.NoError(t, err)
}

func TestUkiIsValidWithAuto(t *testing.T) {
	validUki := Uki{
		Kernels: UkiKernels{
			Auto:    true,
			Kernels: nil, // Auto mode does not require explicit kernel versions
		},
	}

	err := validUki.IsValid()
	assert.NoError(t, err)
}

func TestUkiKernelsIsValidInvalidKernelList(t *testing.T) {
	invalidUki := Uki{
		Kernels: UkiKernels{
			Auto: false,
			Kernels: []string{
				"6.6.51.1-5.azl3",
				"invalid-kernel-version",
			},
		},
	}

	err := invalidUki.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid kernel version at index 1:")
	assert.ErrorContains(t, err, "invalid kernel version format (invalid-kernel-version)")
}

func TestUkiCleanBootDefaultsToFalse(t *testing.T) {
	// Test YAML parsing when cleanBoot is not specified - should default to false
	yamlContent := `kernels: auto`

	var uki Uki
	err := yaml.Unmarshal([]byte(yamlContent), &uki)
	assert.NoError(t, err)
	assert.False(t, uki.CleanBoot, "cleanBoot should default to false when not specified in YAML")

	err = uki.IsValid()
	assert.NoError(t, err)
}

func TestUkiCleanBootSetToTrue(t *testing.T) {
	uki := Uki{
		Kernels: UkiKernels{
			Auto:    true,
			Kernels: nil,
		},
		CleanBoot: true,
	}

	assert.True(t, uki.CleanBoot)
	err := uki.IsValid()
	assert.NoError(t, err)
}

func TestUkiCleanBootSetToFalse(t *testing.T) {
	uki := Uki{
		Kernels: UkiKernels{
			Auto: false,
			Kernels: []string{
				"6.6.51.1-5.azl3",
			},
		},
		CleanBoot: false,
	}

	assert.False(t, uki.CleanBoot)
	err := uki.IsValid()
	assert.NoError(t, err)
}
