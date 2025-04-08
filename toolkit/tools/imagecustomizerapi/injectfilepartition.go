// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

type InjectFilePartition struct {
	MountIdType MountIdentifierType `yaml:"mountIdType" json:"mountIdType,omitempty"`
	Id          string              `yaml:"id" json:"id,omitempty"`
}
