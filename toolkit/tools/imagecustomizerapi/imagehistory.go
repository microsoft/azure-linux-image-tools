// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type ImageHistory string

const (
	ImageHistoryDefault ImageHistory = ""
	ImageHistoryNone    ImageHistory = "none"
)

func (t ImageHistory) IsValid() error {
	switch t {
	case ImageHistoryDefault, ImageHistoryNone:
		// All good.
		return nil

	default:
		return fmt.Errorf("invalid imageHistory value (%s)", t)
	}
}
