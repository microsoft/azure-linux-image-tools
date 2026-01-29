// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMountPointIsValidInvalidIdType(t *testing.T) {
	mountPoint := MountPoint{
		IdType: "bad",
		Path:   "/",
	}

	err := mountPoint.IsValid()
	assert.ErrorContains(t, err, "invalid idType value")
	assert.ErrorContains(t, err, "invalid value (bad)")
}

func TestMountPointIsValidInvalidPath(t *testing.T) {
	mountPoint := MountPoint{
		IdType: MountIdentifierTypeDefault,
		Path:   "",
	}

	err := mountPoint.IsValid()
	assert.ErrorContains(t, err, "invalid path:\npath cannot be empty")
}

func TestMountPointIsValidInvalidOptions(t *testing.T) {
	mountPoint := MountPoint{
		IdType:  MountIdentifierTypeDefault,
		Path:    "/mnt",
		Options: "invalid\toptions",
	}

	err := mountPoint.IsValid()
	assert.ErrorContains(t, err, "options (invalid\toptions) contain spaces, tabs, or newlines and are invalid")
}

func TestValidateBtrfsMountOptions_EmptyString_Pass(t *testing.T) {
	err := validateBtrfsMountOptions("")
	assert.NoError(t, err)
}

func TestValidateBtrfsMountOptions_SingleValidOption_Pass(t *testing.T) {
	err := validateBtrfsMountOptions("compress=zstd")
	assert.NoError(t, err)
}

func TestValidateBtrfsMountOptions_MultipleValidOptions_Pass(t *testing.T) {
	err := validateBtrfsMountOptions("compress=zstd,noatime,nodatacow")
	assert.NoError(t, err)
}

func TestValidateBtrfsMountOptions_SubvolOption_Fail(t *testing.T) {
	err := validateBtrfsMountOptions("subvol=root")
	assert.Error(t, err)
	assert.ErrorContains(t, err, "'subvol=' option is not allowed")
}

func TestValidateBtrfsMountOptions_SubvolidOption_Fail(t *testing.T) {
	err := validateBtrfsMountOptions("subvolid=256")
	assert.Error(t, err)
	assert.ErrorContains(t, err, "'subvolid=' option is not allowed")
}

func TestValidateBtrfsMountOptions_SubvolWithOtherOptions_Fail(t *testing.T) {
	err := validateBtrfsMountOptions("compress=zstd,subvol=home,noatime")
	assert.Error(t, err)
	assert.ErrorContains(t, err, "'subvol=' option is not allowed")
}

func TestValidateBtrfsMountOptions_SubvolidWithOtherOptions_Fail(t *testing.T) {
	err := validateBtrfsMountOptions("noatime,subvolid=256,compress=lzo")
	assert.Error(t, err)
	assert.ErrorContains(t, err, "'subvolid=' option is not allowed")
}
