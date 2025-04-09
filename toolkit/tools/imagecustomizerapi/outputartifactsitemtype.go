// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import "fmt"

type OutputArtifactsItemType string

const (
	OutputArtifactsItemUkis        OutputArtifactsItemType = "ukis"
	OutputArtifactsItemShim        OutputArtifactsItemType = "shim"
	OutputArtifactsItemSystemdBoot OutputArtifactsItemType = "systemd-boot"
	OutputArtifactsItemDefault     OutputArtifactsItemType = ""
)

func (i OutputArtifactsItemType) IsValid() error {
	switch i {
	case OutputArtifactsItemUkis, OutputArtifactsItemShim, OutputArtifactsItemSystemdBoot, OutputArtifactsItemDefault:
		return nil
	default:
		return fmt.Errorf("invalid item value (%v)", i)
	}
}
