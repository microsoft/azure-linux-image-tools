// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type ShrinkPartitionsType string

const (
	ShrinkPartitionsTypeDefault    ShrinkPartitionsType = ""
	ShrinkPartitionsTypeNone       ShrinkPartitionsType = "none"
	ShrinkPartitionsTypeVerityOnly ShrinkPartitionsType = "verity-only"
)

func (t ShrinkPartitionsType) IsValid() error {
	switch t {
	case ShrinkPartitionsTypeDefault, ShrinkPartitionsTypeNone, ShrinkPartitionsTypeVerityOnly:
		// All good.
		return nil

	default:
		return fmt.Errorf("invalid value (%s)", t)
	}
}
