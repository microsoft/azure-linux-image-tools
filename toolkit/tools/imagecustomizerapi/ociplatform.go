// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

type OciPlatform struct {
	OS           string `yaml:"os" json:"os,omitempty"`
	Architecture string `yaml:"architecture" json:"architecture,omitempty"`
}

func (i *OciPlatform) IsValid() error {
	return nil
}

// UnmarshalYAML enables OciPlatform to handle both a shorthand uri and a structured object.
func (p *OciPlatform) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		return p.parseScalar(value.Value)

	default:
		return p.parseStruct(value)
	}
}

func (p *OciPlatform) parseScalar(value string) error {
	// Parse the platform string like the oras command's '--platform' parameter.
	// Future: If variant and OS version are added to the OciPlatform struct, then this logic will need to be updated.
	subs := strings.Split(value, "/")
	if len(subs) > 2 || len(subs) <= 0 {
		return fmt.Errorf("invalid OCI platform string (%s):\nexpected format: OS[/ARCH]", value)
	}

	p.Architecture = runtime.GOARCH
	if len(subs) >= 2 {
		p.Architecture = subs[1]
	}

	p.OS = subs[0]

	return nil
}

func (p *OciPlatform) parseStruct(value *yaml.Node) error {
	// yaml.Node.Decode() doesn't respect the KnownFields() option.
	// So, manually enforce this.
	err := checkKnownFields(value, "OciPlatform", []string{"os", "architecture"})
	if err != nil {
		return err
	}

	// Otherwise, decode as a full struct.
	type IntermediateTypeOciPlatform OciPlatform
	err = value.Decode((*IntermediateTypeOciPlatform)(p))
	if err != nil {
		return fmt.Errorf("failed to parse OciPlatform struct:\n%w", err)
	}

	return nil
}
