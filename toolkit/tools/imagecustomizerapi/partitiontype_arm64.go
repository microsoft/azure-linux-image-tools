// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

var (
	// UUIDs come from:
	// - https://en.wikipedia.org/wiki/GUID_Partition_Table#Partition_type_GUIDs
	// - https://uapi-group.org/specifications/specs/discoverable_partitions_specification/
	partitionTypeToUuidArchDependent = map[PartitionType]string{
		PartitionTypeRoot:       "b921b045-1df0-41c3-af44-4c6f280d3fae",
		PartitionTypeRootVerity: "df3300ce-d69f-4c92-978c-9bfb0f38d820",
		PartitionTypeUsr:        "b0e01050-ee5f-4390-949a-9101b17104e9",
		PartitionTypeUsrVerity:  "6e11a4e7-fbca-4ded-b9e9-e1a512bb664e",
	}
)
