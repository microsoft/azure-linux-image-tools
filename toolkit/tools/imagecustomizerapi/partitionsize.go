// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"encoding/json"
	"fmt"

	"github.com/invopop/jsonschema"
	"gopkg.in/yaml.v3"
)

const (
	PartitionSizeGrow = "grow"
)

type PartitionSizeType int

const (
	PartitionSizeTypeUnset PartitionSizeType = iota
	PartitionSizeTypeGrow
	PartitionSizeTypeExplicit
)

type PartitionSize struct {
	Type PartitionSizeType `json:"type,omitempty"`
	Size DiskSize          `json:"size,omitempty"`
}

func (s *PartitionSize) IsValid() error {
	return nil
}

func (s *PartitionSize) UnmarshalYAML(value *yaml.Node) error {
	var err error

	var stringValue string
	err = value.Decode(&stringValue)
	if err != nil {
		return fmt.Errorf("failed to parse partition size:\n%w", err)
	}

	return parseAndSetPartitionSize(stringValue, s)
}

func (s PartitionSize) MarshalJSON() ([]byte, error) {
	switch s.Type {
	case PartitionSizeTypeGrow:
		return json.Marshal("grow")
	case PartitionSizeTypeExplicit:
		return json.Marshal(s.Size.String())
	default:
		return json.Marshal(nil)
	}
}

func (s *PartitionSize) UnmarshalJSON(data []byte) error {
	var err error

	var stringValue string
	err = json.Unmarshal(data, &stringValue)
	if err != nil {
		return fmt.Errorf("failed to parse partition size:\n%w", err)
	}

	return parseAndSetPartitionSize(stringValue, s)
}

func (PartitionSize) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:    "string",
		Pattern: `^\d+[KMGT]?$|^grow`,
	}
}

func parseAndSetPartitionSize(stringValue string, s *PartitionSize) error {
	switch stringValue {
	case PartitionSizeGrow:
		*s = PartitionSize{
			Type: PartitionSizeTypeGrow,
		}
		return nil
	case "":
		*s = PartitionSize{
			Type: PartitionSizeTypeUnset,
		}
		return nil
	}

	diskSize, err := parseDiskSize(stringValue)
	if err != nil {
		return fmt.Errorf("%w:\nexpected format: grow | <NUM>(K|M|G|T) (e.g. grow, 100M, 1G)", err)
	}

	*s = PartitionSize{
		Type: PartitionSizeTypeExplicit,
		Size: diskSize,
	}
	return nil
}
