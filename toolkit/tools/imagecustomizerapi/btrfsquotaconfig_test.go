// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/stretchr/testify/assert"
)

func TestBtrfsQuotaConfigIsValid_ReferencedLimitZero_Fail(t *testing.T) {
	referencedLimit := DiskSize(0)
	q := BtrfsQuotaConfig{
		ReferencedLimit: &referencedLimit,
	}
	err := q.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "referencedLimit value (0) must be a positive non-zero number")
}

func TestBtrfsQuotaConfigIsValid_ExclusiveLimitZero_Fail(t *testing.T) {
	exclusiveLimit := DiskSize(0)
	q := BtrfsQuotaConfig{
		ExclusiveLimit: &exclusiveLimit,
	}
	err := q.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "exclusiveLimit value (0) must be a positive non-zero number")
}

func TestBtrfsQuotaConfigIsValid_ReferencedLimitUnaligned_Fail(t *testing.T) {
	referencedLimit := DiskSize(1 * diskutils.KiB)
	q := BtrfsQuotaConfig{
		ReferencedLimit: &referencedLimit,
	}
	err := q.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid 'referencedLimit' value:\n")
	assert.ErrorContains(t, err, "size (1 KiB) must be a multiple of 1 MiB")
}

func TestBtrfsQuotaConfigIsValid_ExclusiveLimitUnaligned_Fail(t *testing.T) {
	exclusiveLimit := DiskSize(1 * diskutils.KiB)
	q := BtrfsQuotaConfig{
		ExclusiveLimit: &exclusiveLimit,
	}
	err := q.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid 'exclusiveLimit' value:\n")
	assert.ErrorContains(t, err, "size (1 KiB) must be a multiple of 1 MiB")
}

func TestBtrfsQuotaConfigIsValid_BothLimitsZero_Fail(t *testing.T) {
	referencedLimit := DiskSize(0)
	exclusiveLimit := DiskSize(0)
	q := BtrfsQuotaConfig{
		ReferencedLimit: &referencedLimit,
		ExclusiveLimit:  &exclusiveLimit,
	}
	err := q.IsValid()
	assert.Error(t, err)
	// Should fail on referencedLimit first since it's checked first
	assert.ErrorContains(t, err, "referencedLimit value (0) must be a positive non-zero number")
}

func TestBtrfsQuotaConfigIsValid_Empty_Pass(t *testing.T) {
	q := BtrfsQuotaConfig{}
	err := q.IsValid()
	assert.NoError(t, err)
}

func TestBtrfsQuotaConfigIsValid_ReferencedLimitPositive_Pass(t *testing.T) {
	referencedLimit := DiskSize(1 * diskutils.MiB)
	q := BtrfsQuotaConfig{
		ReferencedLimit: &referencedLimit,
	}
	err := q.IsValid()
	assert.NoError(t, err)
}

func TestBtrfsQuotaConfigIsValid_ExclusiveLimitPositive_Pass(t *testing.T) {
	exclusiveLimit := DiskSize(1 * diskutils.MiB)
	q := BtrfsQuotaConfig{
		ExclusiveLimit: &exclusiveLimit,
	}
	err := q.IsValid()
	assert.NoError(t, err)
}

func TestBtrfsQuotaConfigIsValid_BothLimitsPositive_Pass(t *testing.T) {
	referencedLimit := DiskSize(2 * diskutils.MiB)
	exclusiveLimit := DiskSize(1 * diskutils.MiB)
	q := BtrfsQuotaConfig{
		ReferencedLimit: &referencedLimit,
		ExclusiveLimit:  &exclusiveLimit,
	}
	err := q.IsValid()
	assert.NoError(t, err)
}
