// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import "fmt"

type OutputArtifactsItemType string

const (
	OutputArtifactsItemUkis       OutputArtifactsItemType = "ukis"
	OutputArtifactsItemUkiAddons  OutputArtifactsItemType = "uki-addons"
	OutputArtifactsItemShim       OutputArtifactsItemType = "shim"
	OutputArtifactsItemBootloader OutputArtifactsItemType = "bootloader"
	OutputArtifactsItemVerityHash OutputArtifactsItemType = "verity-hash"
	OutputArtifactsItemDefault    OutputArtifactsItemType = ""
)

func (i OutputArtifactsItemType) IsValid() error {
	switch i {
	case OutputArtifactsItemUkis, OutputArtifactsItemUkiAddons, OutputArtifactsItemShim, OutputArtifactsItemBootloader,
		OutputArtifactsItemVerityHash, OutputArtifactsItemDefault:
		return nil
	default:
		return fmt.Errorf("invalid item value (%v)", i)
	}
}

// IsValidOutputItem validates that the item is a user-selectable output artifact type.
// The uki-addons type is excluded because addon files are automatically included
// when ukis is specified. The uki-addons type is only valid as an inject-files
// metadata type.
func (i OutputArtifactsItemType) IsValidOutputItem() error {
	switch i {
	case OutputArtifactsItemUkis, OutputArtifactsItemShim, OutputArtifactsItemBootloader, OutputArtifactsItemVerityHash,
		OutputArtifactsItemDefault:
		return nil
	default:
		return fmt.Errorf("invalid item value (%v)", i)
	}
}
