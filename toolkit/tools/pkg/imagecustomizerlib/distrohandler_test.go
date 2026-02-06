// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	"github.com/stretchr/testify/assert"
)

func TestDistroHandlerSupportsSELinux_AzureLinuxFedora_True(t *testing.T) {
	handlers := []DistroHandler{
		NewDistroHandlerFromTargetOs(targetos.TargetOsAzureLinux2),
		NewDistroHandlerFromTargetOs(targetos.TargetOsAzureLinux3),
		NewDistroHandlerFromTargetOs(targetos.TargetOsFedora42),
	}

	for _, handler := range handlers {
		assert.True(t, handler.SupportsSELinux())
	}
}

func TestDistroHandlerSupportsSELinux_Ubuntu_False(t *testing.T) {
	handlers := []DistroHandler{
		NewDistroHandlerFromTargetOs(targetos.TargetOsUbuntu2204),
		NewDistroHandlerFromTargetOs(targetos.TargetOsUbuntu2404),
	}

	for _, handler := range handlers {
		assert.False(t, handler.SupportsSELinux())
	}
}
