//go:build prod

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

func (t PasswordType) IsValid() error {
	switch t {
	case PasswordTypeLocked:
		// All good.
		return nil

	case PasswordTypePlainText, PasswordTypeHashed, PasswordTypePlainTextFile, PasswordTypeHashedFile:
		return nil

	default:
		return fmt.Errorf("invalid password type (%s)", t)
	}
}
