// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKernelCommandLineIsValidOk(t *testing.T) {
	cfg := KernelCommandLine{
		Replace: []KernelCmdlineReplace{
			{
				Remove: []string{
					"console",
				},
				Add: []string{
					"console=tty0",
					"console=ttyS0",
				},
			},
		},
		ExtraCommandLine: []string{
			"rd.info",
		},
	}
	err := cfg.IsValid()
	assert.NoError(t, err)
}

func TestKernelCommandLineIsValidReplaceRemoveEmptyValue(t *testing.T) {
	cfg := KernelCommandLine{
		Replace: []KernelCmdlineReplace{
			{
				Remove: []string{
					"",
				},
			},
		},
	}
	err := cfg.IsValid()
	assert.ErrorContains(t, err, "invalid 'replace' value at index 0:\n")
	assert.ErrorContains(t, err, "'remove' value cannot be empty (index=0)")
}
