// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type UkiMode string

const (
	UkiModeUnspecified UkiMode = ""
	UkiModeCreate      UkiMode = "create"
	UkiModePassthrough UkiMode = "passthrough"
)

func (u UkiMode) IsValid() error {
	switch u {
	case UkiModeUnspecified, UkiModeCreate, UkiModePassthrough:
		return nil
	default:
		return fmt.Errorf("invalid uki mode value (%s): must be one of ['', 'create', 'passthrough']", u)
	}
}
