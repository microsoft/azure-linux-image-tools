// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsoIsValidOk(t *testing.T) {
	pxe := Iso{
		AdditionalFiles: []AdditionalFile{
			{
				Source:      "files/a.txt",
				Destination: "/a.txt",
			},
		},
		KernelCommandLine: KernelCommandLine{
			ExtraCommandLine: []string{
				"rd.info",
			},
		},
		InitramfsType: "bootstrap",
	}
	assert.NoError(t, pxe.IsValid())
}

func TestIsoIsValidBadKernelCommandLine(t *testing.T) {
	iso := Iso{
		KernelCommandLine: KernelCommandLine{
			ExtraCommandLine: []string{""},
		},
	}

	err := iso.IsValid()
	assert.ErrorContains(t, err, "invalid kernelCommandLine")
}

func TestIsoIsValidContainsKernelCommandLineReplace(t *testing.T) {
	pxe := Iso{
		KernelCommandLine: KernelCommandLine{
			Replace: []KernelCmdlineReplace{
				{
					Remove: []string{
						"console",
					},
				},
			},
		},
	}
	err := pxe.IsValid()
	assert.ErrorContains(t, err, "invalid kernelCommandLine: 'replace' not supported for 'iso'")
}
