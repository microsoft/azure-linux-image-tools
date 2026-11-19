// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPxeIsValidOk(t *testing.T) {
	pxe := Pxe{
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
		InitramfsType:    "bootstrap",
		BootstrapBaseUrl: "http://my-pxe-server",
	}
	assert.NoError(t, pxe.IsValid())
}

func TestPxeIsValidContainsKernelCommandLineReplace(t *testing.T) {
	pxe := Pxe{
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
	assert.ErrorContains(t, err, "invalid kernelCommandLine: 'replace' not supported for 'pxe'")
}
