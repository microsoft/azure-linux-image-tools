// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package network

import (
	"os"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	logger.InitStderrLog()
	os.Exit(m.Run())
}

func TestCheckNetworkAccess(t *testing.T) {
	err, hasNetworkAccess := CheckNetworkAccess()
	assert.NoError(t, err, "CheckNetworkAccess() failed")
	assert.True(t, hasNetworkAccess, "Test expected to run in an environment with network access.")
}
