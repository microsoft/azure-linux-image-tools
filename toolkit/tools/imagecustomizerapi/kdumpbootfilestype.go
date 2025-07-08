// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type KdumpBootFilesType string

const (
	KdumpBootFilesTypeNone KdumpBootFilesType = ""
	KdumpBootFilesTypeKeep KdumpBootFilesType = "keep"
)

func (t KdumpBootFilesType) IsValid() error {
	switch t {
	case KdumpBootFilesTypeNone, KdumpBootFilesTypeKeep:
		// All good.
		return nil

	default:
		return fmt.Errorf("invalid kdumpBootFilesType value (%v)", t)
	}
}
