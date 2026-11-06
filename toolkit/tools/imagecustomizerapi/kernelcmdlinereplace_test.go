// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKernelCmdlineReplaceIsValidRemoveNotName(t *testing.T) {
	cfg := KernelCmdlineReplace{
		Remove: []string{
			"a=b",
		},
	}
	err := cfg.IsValid()
	assert.ErrorContains(t, err, "'remove' value must be a name only and may not contain a value (index=0, arg='a=b')")
}

func TestKernelCmdlineReplaceIsValidAddEmptyValue(t *testing.T) {
	cfg := KernelCmdlineReplace{
		Add: []string{
			"",
		},
	}
	err := cfg.IsValid()
	assert.ErrorContains(t, err, "'add' value cannot be empty (index=0)")
}

func TestKernelCmdlineReplaceIsValidEmpty(t *testing.T) {
	cfg := KernelCmdlineReplace{}
	err := cfg.IsValid()
	assert.ErrorContains(t, err, "either 'add' or 'remove' must contain at least 1 value")
}
