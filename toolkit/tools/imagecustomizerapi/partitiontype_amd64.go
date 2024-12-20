// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

var (
	// UUIDs come from:
	// - https://en.wikipedia.org/wiki/GUID_Partition_Table#Partition_type_GUIDs
	// - https://uapi-group.org/specifications/specs/discoverable_partitions_specification/
	partitionTypeToUuidArchDependent = map[PartitionType]string{
		PartitionTypeRoot:       "4f68bce3-e8cd-4db1-96e7-fbcaf984b709",
		PartitionTypeRootVerity: "2c7357ed-ebd2-46d9-aec1-23d437ec2bf5",
		PartitionTypeUsr:        "8484680c-9521-48c6-9c11-b0720656f69e",
		PartitionTypeUsrVerity:  "77ff5f63-e7b6-4633-acf4-1565b864c0e6",
	}
)
