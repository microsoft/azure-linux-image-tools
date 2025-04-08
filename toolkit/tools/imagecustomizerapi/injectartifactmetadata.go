// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

type InjectArtifactMetadata struct {
	Partition      InjectFilePartition `yaml:"partition"`
	Destination    string              `yaml:"destination"`
	Source         string              `yaml:"source"`
	UnsignedSource string              `yaml:"unsignedSource""`
}
