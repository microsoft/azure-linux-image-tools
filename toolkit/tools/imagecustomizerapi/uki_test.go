// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	assert.ErrorContains(t, err, "invalid uki kernels: kernel version at index 1 - invalid-kernel-version")
}
