// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/ptrutils"
	"github.com/stretchr/testify/assert"
)

func TestAdditionalFilesIsValidNoDestination(t *testing.T) {
	additionalFiles := AdditionalFileList{
		{
			Destination: "",
			Source:      "a.txt",
		},
	}
	err := additionalFiles.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid value at index 0")
	assert.ErrorContains(t, err, "destination path must not be empty")
}

func TestAdditionalFilesIsValidNoSourceOrContent(t *testing.T) {
	additionalFiles := AdditionalFileList{
		{
			Destination: "/a.txt",
		},
	}
	err := additionalFiles.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid value at index 0")
	assert.ErrorContains(t, err, "must specify either 'source' or 'content'")
}

func TestAdditionalFilesIsValidBothSourceAndContent(t *testing.T) {
	additionalFiles := AdditionalFileList{
		{
			Destination: "/a.txt",
			Source:      "a.txt",
			Content:     ptrutils.PtrTo("abc"),
		},
	}
	err := additionalFiles.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid value at index 0")
	assert.ErrorContains(t, err, "cannot specify both 'source' and 'content'")
}

func TestAdditionalFilesIsValidBadPermissions(t *testing.T) {
	additionalFiles := AdditionalFileList{
		{
			Destination: "/a.txt",
			Source:      "a.txt",
			Permissions: ptrutils.PtrTo(FilePermissions(0o7000)),
		},
	}
	err := additionalFiles.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid value at index 0")
	assert.ErrorContains(t, err, "invalid permissions value")
	assert.ErrorContains(t, err, "0o7000 contains non-permission bits")
}

func TestAdditionalFilesIsValidValidSymlinkMode(t *testing.T) {
	for _, symlinkMode := range []SymlinkMode{
		SymlinkModeUnspecified,
		SymlinkModeDereference,
		SymlinkModePreserve,
	} {
		additionalFiles := AdditionalFileList{
			{
				Destination: "/a.txt",
				Source:      "a.txt",
				SymlinkMode: symlinkMode,
			},
		}
		err := additionalFiles.IsValid()
		assert.NoError(t, err)
	}
}

func TestAdditionalFilesIsValidInvalidSymlinkMode(t *testing.T) {
	additionalFiles := AdditionalFileList{
		{
			Destination: "/a.txt",
			Source:      "a.txt",
			SymlinkMode: SymlinkMode("invalid"),
		},
	}
	err := additionalFiles.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid symlinkMode value")
	assert.ErrorContains(t, err, "must be one of ['', 'dereference', 'preserve']")
}

func TestAdditionalFilesIsValidPreserveWithContent(t *testing.T) {
	additionalFiles := AdditionalFileList{
		{
			Destination: "/a.txt",
			Content:     ptrutils.PtrTo("abc"),
			SymlinkMode: SymlinkModePreserve,
		},
	}
	err := additionalFiles.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "'symlinkMode: preserve' cannot be used with 'content'")
}
