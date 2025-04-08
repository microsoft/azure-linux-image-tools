// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

// InjectArtifactMetadata defines a single artifact to be injected into a partition.
type InjectArtifactMetadata struct {
	// Partition identifies the target partition where this artifact should be injected.
	Partition InjectFilePartition `yaml:"partition" json:"partition,omitempty"`
	// Destination is the absolute path within the mounted partition where the artifact should be placed.
	Destination string `yaml:"destination" json:"destination,omitempty"`
	// Source is the relative path to the signed artifact, resolved relative to the inject-files.yaml file.
	Source string `yaml:"source" json:"source,omitempty"`
	// UnsignedSource is the relative path to the unsigned version of the artifact, also resolved relative to inject-files.yaml.
	// This field is for reference only and is ignored during injection.
	UnsignedSource string `yaml:"unsignedSource" json:"unsignedSource,omitempty"`
}
