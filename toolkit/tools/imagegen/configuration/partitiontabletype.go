// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Parser for the image builder's configuration schemas.

package configuration

// PartitionTableType is either gpt, mbr, or none
type PartitionTableType string

const (
	// PartitionTableTypeGpt selects gpt
	PartitionTableTypeGpt PartitionTableType = "gpt"
	// PartitionTableTypeMbr selects mbr
	PartitionTableTypeMbr PartitionTableType = "mbr"
	// PartitionTableTypeNone selects no partition type
	PartitionTableTypeNone PartitionTableType = ""
)
