// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import "fmt"

type Item string

const (
	ItemUkis        Item = "ukis"
	ItemShim        Item = "shim"
	ItemSystemdBoot Item = "systemdBoot"
	ItemDefault     Item = ""
)

func (i Item) IsValid() error {
	switch i {
	case ItemUkis, ItemShim, ItemSystemdBoot, ItemDefault:
		return nil
	default:
		return fmt.Errorf("invalid item value (%v)", i)
	}
}
