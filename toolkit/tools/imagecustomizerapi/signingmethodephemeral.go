// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

type SigningMethodEphemeral struct {
	PublicKeysPath string `yaml:"publicKeysPath" json:"publicKeysPath,omitempty"`
}

func (m *SigningMethodEphemeral) IsValid() error {
	return nil
}
