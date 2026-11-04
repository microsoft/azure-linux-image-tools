// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package diskutils

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/configuration"
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

// Exercise numeric device constraints independently of verity's current default.
// Inspect real XFS sectors; compatible ext4/Btrfs must retain their original options.
func TestFileSystemLogicalBlockSize(t *testing.T) {
	if testing.Short() {
		t.Skip("Short mode enabled")
	}
	if os.Geteuid() != 0 {
		t.Skip("Test must be run as root")
	}

	optionTokens := func(args []string) []string {
		var tokens []string
		for _, arg := range args {
			tokens = append(tokens, strings.Split(arg, ",")...)
		}
		return tokens
	}
	for _, fsType := range []string{"xfs", "ext4", "btrfs"} {
		for _, logicalBlockSize := range []uint64{0, 1024, 2048, 4096} {
			t.Run(fmt.Sprintf("%s/%d", fsType, logicalBlockSize), func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "filesystem.raw")
				baseline, err := getFileSystemOptions(targetos.TargetOsAzureLinux3, fsType, false, path, 0, 0)
				require.NoError(t, err)
				args, err := getFileSystemOptions(targetos.TargetOsAzureLinux3, fsType, false, path, 0, logicalBlockSize)
				require.NoError(t, err)
				if fsType != "xfs" || logicalBlockSize == 0 {
					assert.ElementsMatch(t, optionTokens(baseline), optionTokens(args))
				}
				if fsType != "xfs" {
					return
				}
				index := slices.Index(args, "-s")
				if logicalBlockSize == 0 {
					assert.Equal(t, -1, index)
				} else {
					require.NotEqual(t, -1, index)
					require.Less(t, index+1, len(args))
					assert.Equal(t, fmt.Sprintf("size=%d", logicalBlockSize), args[index+1])
				}

				file, err := os.Create(path)
				require.NoError(t, err)
				require.NoError(t, file.Truncate(512*MiB))
				require.NoError(t, file.Close())
				_, err = formatSinglePartition(targetos.TargetOsAzureLinux3, path, path,
					configuration.Partition{FsType: fsType, LogicalBlockSize: logicalBlockSize}, false)
				require.NoError(t, err)
				file, err = os.Open(path)
				require.NoError(t, err)
				sector := make([]byte, 2)
				_, err = file.ReadAt(sector, 102) // XFS sb_sectsize.
				require.NoError(t, file.Close())
				require.NoError(t, err)
				expected := logicalBlockSize
				if expected == 0 {
					expected = 512
				}
				assert.Equal(t, expected, uint64(binary.BigEndian.Uint16(sector)))
			})
		}
	}
}
