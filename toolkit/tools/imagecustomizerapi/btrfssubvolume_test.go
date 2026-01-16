// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBtrfsSubvolumeIsValid_InvalidPath_Fail(t *testing.T) {
	s := BtrfsSubvolume{
		Path: "", // Invalid: empty
	}
	err := s.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid path")
}

func TestBtrfsSubvolumeIsValid_InvalidMountPointPath_Fail(t *testing.T) {
	s := BtrfsSubvolume{
		Path: "root",
		MountPoint: &MountPoint{
			Path: "var", // Invalid: not absolute
		},
	}
	err := s.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid mountPoint")
}

func TestBtrfsSubvolumeIsValid_InvalidBtrfsMountPointOptions_Fail(t *testing.T) {
	s := BtrfsSubvolume{
		Path: "root",
		MountPoint: &MountPoint{
			Path:    "/",
			Options: "subvol=/root",
		},
	}
	err := s.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid mountPoint.options")
}

func TestBtrfsSubvolumeIsValid_InvalidQuota_Fail(t *testing.T) {
	zeroSize := DiskSize(0)
	s := BtrfsSubvolume{
		Path: "root",
		Quota: &BtrfsQuotaConfig{
			ReferencedLimit: &zeroSize,
		},
	}
	err := s.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid quota")
}

func TestBtrfsSubvolumeIsValid_MinimalValid_Pass(t *testing.T) {
	s := BtrfsSubvolume{
		Path: "root",
	}
	err := s.IsValid()
	assert.NoError(t, err)
}

func TestBtrfsSubvolumeIsValid_ValidWithAllFields_Pass(t *testing.T) {
	refLimit := DiskSize(1024 * 1024 * 1024) // 1 GiB
	exLimit := DiskSize(512 * 1024 * 1024)   // 512 MiB
	s := BtrfsSubvolume{
		Path: "root",
		MountPoint: &MountPoint{
			Path:    "/",
			Options: "compress=zstd,noatime",
			IdType:  MountIdentifierTypeUuid,
		},
		Quota: &BtrfsQuotaConfig{
			ReferencedLimit: &refLimit,
			ExclusiveLimit:  &exLimit,
		},
	}
	err := s.IsValid()
	assert.NoError(t, err)
}

func TestValidateSubvolumePath_SimpleName_Pass(t *testing.T) {
	err := validateSubvolumePath("root")
	assert.NoError(t, err)
}

func TestValidateSubvolumePath_NestedPath_Pass(t *testing.T) {
	err := validateSubvolumePath("root/home")
	assert.NoError(t, err)
}

func TestValidateSubvolumePath_DeeplyNestedPath_Pass(t *testing.T) {
	err := validateSubvolumePath("root/home/user/data")
	assert.NoError(t, err)
}

func TestValidateSubvolumePath_NameWithHyphen_Pass(t *testing.T) {
	err := validateSubvolumePath("my-subvol")
	assert.NoError(t, err)
}

func TestValidateSubvolumePath_NameWithUnderscore_Pass(t *testing.T) {
	err := validateSubvolumePath("my_subvol")
	assert.NoError(t, err)
}

func TestValidateSubvolumePath_AtPrefix_Pass(t *testing.T) {
	err := validateSubvolumePath("@root")
	assert.NoError(t, err)
}

func TestValidateSubvolumePath_NestedWithAtPrefix_Pass(t *testing.T) {
	err := validateSubvolumePath("@snapshots/daily")
	assert.NoError(t, err)
}

func TestValidateSubvolumePath_EmptyString_Fail(t *testing.T) {
	err := validateSubvolumePath("")
	assert.Error(t, err)
	assert.ErrorContains(t, err, "path must not be empty")
}

func TestValidateSubvolumePath_StartsWithSlash_Fail(t *testing.T) {
	err := validateSubvolumePath("/root")
	assert.Error(t, err)
	assert.ErrorContains(t, err, "path must not start with '/'")
}

func TestValidateSubvolumePath_EndsWithSlash_Fail(t *testing.T) {
	err := validateSubvolumePath("root/")
	assert.Error(t, err)
	assert.ErrorContains(t, err, "path must not end with '/'")
}

func TestValidateSubvolumePath_DoubleSlash_Fail(t *testing.T) {
	err := validateSubvolumePath("root//home")
	assert.Error(t, err)
	assert.ErrorContains(t, err, "path must not contain double slashes")
}

func TestValidateSubvolumePath_ContainsDoubleDot_Fail(t *testing.T) {
	err := validateSubvolumePath("root/../home")
	assert.Error(t, err)
	assert.ErrorContains(t, err, "path must not contain '..' components")
}

func TestValidateSubvolumePath_OnlyDoubleDot_Fail(t *testing.T) {
	err := validateSubvolumePath("..")
	assert.Error(t, err)
	assert.ErrorContains(t, err, "path must not contain '..' components")
}

func TestValidateSubvolumePath_ContainsSingleDot_Fail(t *testing.T) {
	err := validateSubvolumePath("root/./home")
	assert.Error(t, err)
	assert.ErrorContains(t, err, "path must not contain '.' components")
}

func TestValidateSubvolumePath_OnlySingleDot_Fail(t *testing.T) {
	err := validateSubvolumePath(".")
	assert.Error(t, err)
	assert.ErrorContains(t, err, "path must not contain '.' components")
}
