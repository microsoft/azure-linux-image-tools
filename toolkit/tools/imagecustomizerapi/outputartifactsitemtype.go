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
	case OutputArtifactsItemUkis, OutputArtifactsItemShim, OutputArtifactsItemBootloader, OutputArtifactsItemVerityHash,
		OutputArtifactsItemDefault:
		return nil
	case OutputArtifactsItemUkiAddons:
		return fmt.Errorf("invalid item value (%v); uki-addons are automatically included with ukis", i)
	default:
		return fmt.Errorf("invalid item value (%v)", i)
	}
}
