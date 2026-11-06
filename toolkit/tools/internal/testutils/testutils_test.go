// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package testutils

import (
	"strings"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectVerityRequiresRootHash(t *testing.T) {
	connection := imageconnection.NewImageConnection()
	_, err := connectVerity(connection, 3, VerityMount{}, make(map[verityMountKey]string))
	require.ErrorContains(t, err, "requires a root hash")
}

func TestConnectVerityReusesMapping(t *testing.T) {
	connection := imageconnection.NewImageConnection()
	verity := VerityMount{HashPartitionNum: 4, RootHash: strings.Repeat("a", 64)}
	devices := map[verityMountKey]string{
		{PartitionNum: 3, VerityMount: verity}: "/dev/mapper/test",
	}
	path, err := connectVerity(connection, 3, verity, devices)
	require.NoError(t, err)
	assert.Equal(t, "/dev/mapper/test", path)
}
