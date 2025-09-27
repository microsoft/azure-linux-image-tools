// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Parser for the image builder's configuration schemas.

package configuration

// Disk holds the disk partitioning, formatting and size information.
// It may also define artifacts generated for each disk.
type Disk struct {
	PartitionTableType PartitionTableType `json:"PartitionTableType"`
	MaxSize            uint64             `json:"MaxSize"`
	TargetDisk         TargetDisk         `json:"TargetDisk"`
	Artifacts          []Artifact         `json:"Artifacts"`
	Partitions         []Partition        `json:"Partitions"`
	RawBinaries        []RawBinary        `json:"RawBinaries"`
}
