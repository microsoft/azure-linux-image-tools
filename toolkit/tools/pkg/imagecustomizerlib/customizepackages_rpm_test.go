// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRpmParseNevraEpoch(t *testing.T) {
	name, epoch, version, release, arch, err := rpmParseNevra("nano-0:8.7.1-2.fc44.x86_64")
	assert.NoError(t, err)
	assert.Equal(t, "nano", name)
	assert.Equal(t, "0", epoch)
	assert.Equal(t, "8.7.1", version)
	assert.Equal(t, "2.fc44", release)
	assert.Equal(t, "x86_64", arch)
}

func TestRpmParseNevraDashes(t *testing.T) {
	name, epoch, version, release, arch, err := rpmParseNevra("systemd-boot-unsigned-259.7-1.fc44.x86_64")
	assert.NoError(t, err)
	assert.Equal(t, "systemd-boot-unsigned", name)
	assert.Equal(t, "", epoch)
	assert.Equal(t, "259.7", version)
	assert.Equal(t, "1.fc44", release)
	assert.Equal(t, "x86_64", arch)
}

func TestRpmParseNevraBad(t *testing.T) {
	_, _, _, _, _, err := rpmParseNevra("systemd-259.7")
	assert.ErrorIs(t, err, ErrPackagesInvalidRpmNevra)
}

func TestRpmParseEvrEpoch(t *testing.T) {
	epoch, version, release, err := rpmParseEvr("0:8.7.1-2.fc44")
	assert.NoError(t, err)
	assert.Equal(t, "0", epoch)
	assert.Equal(t, "8.7.1", version)
	assert.Equal(t, "2.fc44", release)
}

func TestRpmParseEvrNoEpoch(t *testing.T) {
	epoch, version, release, err := rpmParseEvr("259.7-1.fc44")
	assert.NoError(t, err)
	assert.Equal(t, "", epoch)
	assert.Equal(t, "259.7", version)
	assert.Equal(t, "1.fc44", release)
}

func TestRpmParseEvrBad(t *testing.T) {
	_, _, _, err := rpmParseEvr("259.7")
	assert.ErrorIs(t, err, ErrPackagesInvalidRpmEvr)
}
