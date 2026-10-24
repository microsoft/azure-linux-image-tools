//go:build prod

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAclRootVerityPreservesProdRestrictions(t *testing.T) {
	assert.ErrorContains(t, PasswordTypePlainText.IsValid(), "only supported in dev builds")
}
