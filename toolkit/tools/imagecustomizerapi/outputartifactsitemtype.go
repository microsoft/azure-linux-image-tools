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
