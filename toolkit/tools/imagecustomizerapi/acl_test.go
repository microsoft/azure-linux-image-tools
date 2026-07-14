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
	assert.ErrorContains(t, err, "at least one partition")
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
