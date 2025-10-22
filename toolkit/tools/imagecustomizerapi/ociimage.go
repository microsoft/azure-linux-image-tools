// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"

	"gopkg.in/yaml.v3"
	"oras.land/oras-go/v2/registry"
)

type OciImage struct {
	Uri      string       `yaml:"uri" json:"uri,omitempty"`
	Platform *OciPlatform `yaml:"platform" json:"platform,omitempty"`
}

func (oi *OciImage) IsValid() error {
	_, err := registry.ParseReference(oi.Uri)
	if err != nil {
		return fmt.Errorf("invalid 'uri' field:\n%w", err)
	}

	if oi.Platform != nil {
		err = oi.Platform.IsValid()
		if err != nil {
			return fmt.Errorf("invalid 'platform' field:\n%w", err)
		}
	}

	return nil
}

// UnmarshalYAML enables OciImage to handle both a shorthand uri and a structured object.
func (oi *OciImage) UnmarshalYAML(value *yaml.Node) error {
	// Check if the node is a scalar (i.e., single uri string).
	if value.Kind == yaml.ScalarNode {
		// Treat scalar value as the Uri directly.
		oi.Uri = value.Value
		return nil
	}

	// yaml.Node.Decode() doesn't respect the KnownFields() option.
	// So, manually enforce this.
	err := checkKnownFields(value, "OciImage", []string{"uri", "platform"})
	if err != nil {
		return err
	}

	// Otherwise, decode as a full struct.
	type IntermediateTypeOciImage OciImage
	err = value.Decode((*IntermediateTypeOciImage)(oi))
	if err != nil {
		return fmt.Errorf("failed to parse OciImage struct:\n%w", err)
	}

	return nil
}
