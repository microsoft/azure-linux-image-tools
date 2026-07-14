// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAclIsValidUsrOnly(t *testing.T) {
	acl := Acl{
		Usr: &AclPartitionGrow{Size: 2 * diskGiB},
	}
	err := acl.IsValid()
	assert.NoError(t, err)
}

func TestAclIsValidEspOnly(t *testing.T) {
	acl := Acl{
		Esp: &AclPartitionGrow{Size: 512 * diskMiB},
	}
	err := acl.IsValid()
	assert.NoError(t, err)
}

func TestAclIsValidUsrAndEsp(t *testing.T) {
	acl := Acl{
		Usr: &AclPartitionGrow{Size: 2 * diskGiB},
		Esp: &AclPartitionGrow{Size: 512 * diskMiB},
	}
	err := acl.IsValid()
	assert.NoError(t, err)
}

func TestAclIsValidEmpty(t *testing.T) {
	acl := Acl{}
	err := acl.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "must specify at least one")
}

func TestAclIsValidZeroSize(t *testing.T) {
	acl := Acl{
		Usr: &AclPartitionGrow{Size: 0},
	}
	err := acl.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "'size' must be specified")
}

func TestAclPartitionGrowIsValidZeroSize(t *testing.T) {
	grow := AclPartitionGrow{Size: 0}
	err := grow.IsValid()
	assert.Error(t, err)
}

const (
	diskMiB DiskSize = 1024 * 1024
	diskGiB DiskSize = 1024 * diskMiB
)

func TestAclIsValidOemIdOnly(t *testing.T) {
	acl := Acl{OemId: "metal"}
	err := acl.IsValid()
	assert.NoError(t, err)
}

func TestAclIsValidUsrAndOemId(t *testing.T) {
	acl := Acl{
		Usr:   &AclPartitionGrow{Size: 2 * diskGiB},
		OemId: "metal",
	}
	err := acl.IsValid()
	assert.NoError(t, err)
}

func TestAclIsValidOemIdInvalid(t *testing.T) {
	for _, bad := range []string{"Metal", "met al", "metal!", "AZURE", "me_tal"} {
		acl := Acl{OemId: bad}
		err := acl.IsValid()
		assert.Errorf(t, err, "oemId %q should be rejected", bad)
		if err != nil {
			assert.ErrorContains(t, err, "invalid 'acl.oemId'")
		}
	}
}

func TestAclIsValidOemIdValidValues(t *testing.T) {
	for _, good := range []string{"metal", "azure", "qemu", "gce", "ec2"} {
		acl := Acl{OemId: good}
		assert.NoErrorf(t, acl.IsValid(), "oemId %q should be accepted", good)
	}
}
