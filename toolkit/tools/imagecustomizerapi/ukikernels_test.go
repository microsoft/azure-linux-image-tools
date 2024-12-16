// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestUkiKernelsUnmarshalYAML_Auto(t *testing.T) {
	yamlContent := `kernels: "auto"`

	var kernels UkiKernels
	err := yaml.Unmarshal([]byte(yamlContent), &kernels)
	assert.NoError(t, err)
	assert.True(t, kernels.Auto)
	assert.Nil(t, kernels.Kernels)
}

func TestUkiKernelsUnmarshalYAML_List(t *testing.T) {
	yamlContent := `
kernels:
  - "6.6.51.1-5.azl3"
  - "5.10.120-4.custom"
`

	var kernels UkiKernels
	err := yaml.Unmarshal([]byte(yamlContent), &kernels)
	assert.NoError(t, err)
	assert.False(t, kernels.Auto)
	assert.Equal(t, []string{"6.6.51.1-5.azl3", "5.10.120-4.custom"}, kernels.Kernels)
}

func TestUkiKernelsUnmarshalYAML_Invalid(t *testing.T) {
	yamlContent := `kernels: invalid`

	var kernels UkiKernels
	err := yaml.Unmarshal([]byte(yamlContent), &kernels)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid YAML structure for 'kernels': must be either 'auto' or a list of kernel names")
}

func TestUkiKernelsIsValid_EmptyList(t *testing.T) {
	invalidKernels := UkiKernels{
		Auto:    false,
		Kernels: []string{},
	}

	err := invalidKernels.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "must specify either 'auto' or a non-empty list of kernel names")
}

func TestUkiKernelsIsValid_AutoAndList(t *testing.T) {
	invalidKernels := UkiKernels{
		Auto:    true,
		Kernels: []string{"6.6.51.1-5.azl3"},
	}

	err := invalidKernels.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "'auto' cannot coexist with a list of kernel names")
}
