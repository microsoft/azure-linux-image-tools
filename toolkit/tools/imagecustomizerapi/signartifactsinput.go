// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

type SignArtifactsInput struct {
	ArtifactsPath string `yaml:"artifactsPath" json:"artifactsPath,omitempty"`
}

func (i *SignArtifactsInput) IsValid() error {
	return nil
}
