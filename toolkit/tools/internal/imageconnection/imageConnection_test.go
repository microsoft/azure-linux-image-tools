// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imageconnection

import (
	"errors"
	"os"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	logger.InitStderrLog()
	os.Exit(m.Run())
}

func TestOwnedResourcesCloseInReverseOrder(t *testing.T) {
	connection := NewImageConnection()
	var order []int
	connection.AddOwnedCleanup(func() error { order = append(order, 1); return nil })
	connection.AddOwnedCleanup(func() error { order = append(order, 2); return nil })
	connection.Close()
	connection.Close()
	assert.Equal(t, []int{2, 1}, order)
}

func TestOwnedResourceCleanupCanBeRetried(t *testing.T) {
	connection := NewImageConnection()
	expected := errors.New("cleanup failed")
	attempts := 0
	connection.AddOwnedCleanup(func() error {
		attempts++
		if attempts == 1 {
			return expected
		}
		return nil
	})
	require.ErrorIs(t, connection.CleanClose(), expected)
	require.NoError(t, connection.CleanClose())
	require.NoError(t, connection.CleanClose())
	assert.Equal(t, 2, attempts)
}

func TestBestEffortCleanupContinuesAfterFailure(t *testing.T) {
	connection := NewImageConnection()
	var order []int
	attempts := 0
	connection.AddOwnedCleanup(func() error { order = append(order, 1); return nil })
	connection.AddOwnedCleanup(func() error {
		order = append(order, 2)
		attempts++
		if attempts == 1 {
			return errors.New("cleanup failed")
		}
		return nil
	})
	connection.AddOwnedCleanup(func() error { order = append(order, 3); return nil })
	connection.Close()
	assert.Equal(t, []int{3, 2, 1}, order)
	connection.Close()
	require.NoError(t, connection.CleanClose())
	assert.Equal(t, []int{3, 2, 1, 2}, order)
}

func TestStrictCleanupRetainsDependenciesOnFailure(t *testing.T) {
	connection := NewImageConnection()
	var order []int
	attempts := 0
	connection.AddOwnedCleanup(func() error { order = append(order, 1); return nil })
	connection.AddOwnedCleanup(func() error {
		order = append(order, 2)
		attempts++
		if attempts == 1 {
			return errors.New("cleanup failed")
		}
		return nil
	})
	require.Error(t, connection.CleanClose())
	assert.Equal(t, []int{2}, order)
	require.NoError(t, connection.CleanClose())
	assert.Equal(t, []int{2, 2, 1}, order)
}
