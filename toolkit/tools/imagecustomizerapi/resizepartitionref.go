// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"

	"go.yaml.in/yaml/v3"
)

const (
	ResizePartitionRefLast = "last"
)

type ResizePartitionRef struct {
	// Specifies the type of ID used to identify the partition.
	IdType ResizePartitionRefType `yaml:"idType" json:"idType,omitempty"`
	// Specifies the id of the partition, interpreted according to the idType field.
	Id string `yaml:"id" json:"id,omitempty"`
}

// UnmarshalYAML enables MountPoint to handle both a shorthand path and a structured object.
func (r *ResizePartitionRef) UnmarshalYAML(value *yaml.Node) error {
	// Check if the node is a scalar.
	if value.Kind == yaml.ScalarNode {
		// Handle syntactic sugar forms.
		switch value.Value {
		case ResizePartitionRefLast:
			*r = ResizePartitionRef{
				IdType: ResizePartitionRefTypeLast,
			}

		default:
			return fmt.Errorf("invalid ResizePartitionRef value (%s)", value.Value)
		}
		return nil
	}

	// yaml.Node.Decode() doesn't respect the KnownFields() option.
	// So, manually enforce this.
	err := checkKnownFields(value, "ResizePartitionRef", []string{"idType", "id"})
	if err != nil {
		return err
	}

	// Otherwise, decode as a full MountPoint struct.
	type IntermediateType ResizePartitionRef
	err = value.Decode((*IntermediateType)(r))
	if err != nil {
		return fmt.Errorf("failed to parse ResizePartitionRef struct:\n%w", err)
	}
	return nil
}

func (r ResizePartitionRef) IsValid() error {
	if err := r.IdType.IsValid(); err != nil {
		return fmt.Errorf("invalid 'idType' value:\n%w", err)
	}

	switch r.IdType {
	case ResizePartitionRefTypeLast:
		if r.Id != "" {
			return fmt.Errorf("'id' value must be empty for 'idType' of 'last'")
		}

	default:
		if r.Id != "" {
			return fmt.Errorf("'id' value must not be empty for 'idType' of '%s'", r.IdType)
		}
	}

	return nil
}
