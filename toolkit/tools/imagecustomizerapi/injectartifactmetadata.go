// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import "fmt"

// InjectArtifactMetadata defines a single artifact to be injected into a partition.
type InjectArtifactMetadata struct {
	// Partition identifies the target partition where this artifact should be injected.
	Partition InjectFilePartition `yaml:"partition" json:"partition,omitempty"`
	// Destination is the absolute path within the mounted partition where the artifact should be placed.
	Destination string `yaml:"destination" json:"destination,omitempty"`
	// Source is the relative path to the artifact file, resolved relative to the inject-files.yaml file.
	// The user should replace the unsigned file at this path with the signed version before injection.
	Source string `yaml:"source" json:"source,omitempty"`
	// Type indicates the artifact type (e.g., "ukis", "shim", "systemd-boot", "verity-hash").
	Type OutputArtifactsItemType `yaml:"type" json:"type,omitempty"`
}

func (iam *InjectArtifactMetadata) IsValid() error {
	if iam.Source == "" || iam.Destination == "" {
		return fmt.Errorf("source or destination is empty")
	}

	err := iam.Partition.IsValid()
	if err != nil {
		return fmt.Errorf("invalid partition:\n%w", err)
	}

	return nil
}
