// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package diskutils

import (
	"fmt"
	"slices"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			assert.Truef(t, a.Lt(b), "%s not ordered correctly: %v must be < %v", distro, a, b)
		}
	}
}

// Check our sector-size arguments without invoking mkfs.xfs.
func TestGetXfsFileSystemOptionsLogicalBlockSize(t *testing.T) {
	for _, test := range []struct {
		logicalBlockSize uint64
		expectedSector   string
	}{
		{0, ""},
		{512, "size=512"},
		{1024, "size=1024"},
		{2048, "size=2048"},
		{4096, "size=4096"},
	} {
		t.Run(fmt.Sprint(test.logicalBlockSize), func(t *testing.T) {
			args, err := getXfsFileSystemOptions(version.Version{6, 6}, version.Version{6, 6},
				fileSystemsOptions{Xfs: azl3XfsOptions}, false, "/dev/test", 0, test.logicalBlockSize)
			require.NoError(t, err)
			sector := ""
			if index := slices.Index(args, "-s"); index >= 0 {
				require.Less(t, index+1, len(args))
				sector = args[index+1]
			}
			assert.Equal(t, test.expectedSector, sector)
			assert.NotContains(t, args, "-b")
		})
	}
}
