// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package toolschroot

import (
	"errors"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	"github.com/stretchr/testify/assert"
)

func TestResolveSupportedDistros(t *testing.T) {
	cases := []struct {
		name        string
		target      targetos.TargetOs
		expectedRef string
	}{
		{"AzureLinux3", targetos.TargetOsAzureLinux3, "mcr.microsoft.com/azurelinux/base/core:3.0"},
		{"AzureLinux4UsesBetaRepo", targetos.TargetOsAzureLinux4, "mcr.microsoft.com/azurelinux-beta/base/core:4.0"},
		{"AzureContainerLinux3MapsToAzureLinux3Base", targetos.TargetOsAzureContainerLinux3, "mcr.microsoft.com/azurelinux/base/core:3.0"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref, err := Resolve(tc.target)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedRef, ref)
		})
	}
}

func TestResolveUnsupported(t *testing.T) {
	cases := []struct {
		name   string
		target targetos.TargetOs
	}{
		{"Ubuntu2204", targetos.TargetOsUbuntu2204},
		{"UnknownDistro", targetos.New("rhel", "9")},
		{"EmptyDistro", targetos.TargetOs{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref, err := Resolve(tc.target)
			assert.Empty(t, ref)
			assert.True(t, errors.Is(err, ErrUnsupportedDistro), "got %v", err)
		})
	}
}
