// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import "fmt"

type OutputArtifactsItemType string

const (
	OutputArtifactsItemUkis        OutputArtifactsItemType = "ukis"
	OutputArtifactsItemUkiAddons   OutputArtifactsItemType = "uki-addons"
	OutputArtifactsItemShim        OutputArtifactsItemType = "shim"
	OutputArtifactsItemSystemdBoot OutputArtifactsItemType = "systemd-boot"
	OutputArtifactsItemVerityHash  OutputArtifactsItemType = "verity-hash"
	OutputArtifactsItemDefault     OutputArtifactsItemType = ""
)

func (i OutputArtifactsItemType) IsValid() error {
	switch i {
	case OutputArtifactsItemUkis, OutputArtifactsItemUkiAddons, OutputArtifactsItemShim, OutputArtifactsItemSystemdBoot,
		OutputArtifactsItemVerityHash, OutputArtifactsItemDefault:
		return nil
	default:
		return fmt.Errorf("invalid item value (%v)", i)
	}
}
