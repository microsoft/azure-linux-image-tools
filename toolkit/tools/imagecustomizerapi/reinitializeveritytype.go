// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type ReinitializeVerityType string

const (
	ReinitializeVerityTypeDefault ReinitializeVerityType = ""
	ReinitializeVerityTypeNone    ReinitializeVerityType = "none"
	ReinitializeVerityTypeAll     ReinitializeVerityType = "all"
)

func (t ReinitializeVerityType) IsValid() error {
	switch t {
	case ReinitializeVerityTypeDefault, ReinitializeVerityTypeNone, ReinitializeVerityTypeAll:
		// All good.
		return nil

	default:
		return fmt.Errorf("invalid value (%s)", t)
	}
}
