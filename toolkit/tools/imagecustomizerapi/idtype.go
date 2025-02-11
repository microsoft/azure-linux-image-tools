// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type IdType string

const (
	IdTypePartLabel IdType = "part-label"
)

func (i IdType) IsValid() error {
	switch i {
	case IdTypePartLabel:
		// All good.
		return nil

	default:
		return fmt.Errorf("invalid idType value (%v)", i)
	}
}
