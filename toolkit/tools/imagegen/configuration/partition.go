// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Parser for the image builder's configuration schemas.

package configuration

const (
	EFIPartitionType    = "efi"
	LegacyPartitionType = "legacy"
)

// Partition defines the size, name and file system type
// for a partition.
// "Start" and "End" fields define the offset from the beginning of the disk in MBs.
// An "End" value of 0 will determine the size of the partition using the next
// partition's start offset or the value defined by "MaxSize", if this is the last
// partition on the disk.
// "Grow" tells the logical volume to fill up any available space (**Only used for
// kickstart-style unattended installation**)
type Partition struct {
	FsType    string          `json:"FsType"`
	Type      string          `json:"Type"`
	TypeUUID  string          `json:"TypeUUID"`
	ID        string          `json:"ID"`
	Name      string          `json:"Name"`
	End       uint64          `json:"End"`
	Start     uint64          `json:"Start"`
	Flags     []PartitionFlag `json:"Flags"`
	Artifacts []Artifact      `json:"Artifacts"`
}

// HasFlag returns true if a given partition has a specific flag set.
func (p *Partition) HasFlag(flag PartitionFlag) bool {
	for _, f := range p.Flags {
		if f == flag {
			return true
		}
	}
	return false
}
