// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package diskutils

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/version"
	"github.com/stretchr/testify/assert"
)

func TestGetFileSystemOptionsForTargetOsExactMatch(t *testing.T) {
	verOptions, err := getFileSystemOptionsForTargetOs(targetos.New("azurelinux", "3.0"))
	assert.NoError(t, err)
	assert.Equal(t, version.Version{3, 0}, verOptions.Version)
}

func TestGetFileSystemOptionsForTargetOsInvalidVersion(t *testing.T) {
	verOptions, err := getFileSystemOptionsForTargetOs(targetos.New("azurelinux", "3.0a"))
	assert.NoError(t, err)
	assert.Equal(t, version.Version{2, 0}, verOptions.Version)
}

func TestGetFileSystemOptionsForTargetOsTooOld(t *testing.T) {
	verOptions, err := getFileSystemOptionsForTargetOs(targetos.New("azurelinux", "1.0"))
	assert.NoError(t, err)
	assert.Equal(t, version.Version{2, 0}, verOptions.Version)
}

func TestGetFileSystemOptionsForTargetOsUnsupportedDistro(t *testing.T) {
	_, err := getFileSystemOptionsForTargetOs(targetos.New("lyrebird", "1.0"))
	assert.ErrorContains(t, err, "unknown target OS (distro='lyrebird', version='1.0')")
}

func TestDistroFileSystemsOptionsOrdering(t *testing.T) {
	for distro, verFsOptions := range distroFileSystemsOptions {
		for i := 0; i < len(verFsOptions)-1; i++ {
			a := verFsOptions[i].Version
			b := verFsOptions[i+1].Version
			assert.Truef(t, a.Le(b), "%s not ordered correctly: %v must be <= %v", distro, a, b)
		}
	}
}
