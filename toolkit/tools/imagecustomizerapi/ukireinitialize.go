// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type UkiReinitialize string

const (
	UkiReinitializeUnspecified UkiReinitialize = ""
	UkiReinitializePassthrough UkiReinitialize = "passthrough"
	UkiReinitializeRefresh     UkiReinitialize = "refresh"
)

func (u UkiReinitialize) IsValid() error {
	switch u {
	case UkiReinitializeUnspecified, UkiReinitializePassthrough, UkiReinitializeRefresh:
		return nil
	default:
		return fmt.Errorf("invalid uki reinitialize value (%s): must be one of ['', 'passthrough', 'refresh']", u)
	}
}
