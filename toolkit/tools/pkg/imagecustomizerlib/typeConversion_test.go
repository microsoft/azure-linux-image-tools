// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/stretchr/testify/assert"
)

func TestCalcIsBootPartition(t *testing.T) {
	assert.True(t, calcIsBootPartition(
		imagecustomizerapi.FileSystem{
			MountPoint: &imagecustomizerapi.MountPoint{
				Path: "/boot",
			},
		},
		[]imagecustomizerapi.FileSystem{},
	))
	assert.True(t, calcIsBootPartition(
		imagecustomizerapi.FileSystem{
			MountPoint: &imagecustomizerapi.MountPoint{
				Path: "/boot/efi",
			},
		},
		[]imagecustomizerapi.FileSystem{},
	))
	assert.True(t, calcIsBootPartition(
		imagecustomizerapi.FileSystem{
			MountPoint: &imagecustomizerapi.MountPoint{
				Path: "/",
			},
		},
		[]imagecustomizerapi.FileSystem{},
	))
	assert.False(t, calcIsBootPartition(
		imagecustomizerapi.FileSystem{
			MountPoint: &imagecustomizerapi.MountPoint{
				Path: "/",
			},
		},
		[]imagecustomizerapi.FileSystem{
			{
				MountPoint: &imagecustomizerapi.MountPoint{
					Path: "/boot",
				},
			},
		},
	))
	assert.False(t, calcIsBootPartition(
		imagecustomizerapi.FileSystem{
			MountPoint: &imagecustomizerapi.MountPoint{
				Path: "/var",
			},
		},
		[]imagecustomizerapi.FileSystem{},
	))
}
