// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSnapshotTime_EmptyIsValid(t *testing.T) {
	s := PackageSnapshotTime("")
	err := s.IsValid()
	assert.NoError(t, err)
}

func TestSnapshotTime_ValidISODate(t *testing.T) {
	s := PackageSnapshotTime("2024-05-20")
	err := s.IsValid()
	assert.NoError(t, err)
}

func TestSnapshotTime_ValidRFC3339(t *testing.T) {
	s := PackageSnapshotTime("2024-05-20T12:34:56Z")
	err := s.IsValid()
	assert.NoError(t, err)
}

func TestSnapshotTime_InvalidDateFormat(t *testing.T) {
	s := PackageSnapshotTime("20-05-2024")
	err := s.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid snapshot time format")
}

func TestSnapshotTime_InvalidTimestampFormat(t *testing.T) {
	s := PackageSnapshotTime("2024-05-20 23:59:59")
	err := s.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid snapshot time format")
}
