// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imageconnection

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
