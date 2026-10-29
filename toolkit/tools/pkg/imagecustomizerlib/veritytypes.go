// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
)

type verityDeviceMetadata struct {
	name                  string
	rootHash              string
	dataPartUuid          string
	hashPartUuid          string
	dataDeviceMountIdType imagecustomizerapi.MountIdentifierType
	hashDeviceMountIdType imagecustomizerapi.MountIdentifierType
	corruptionOption      imagecustomizerapi.CorruptionOption
	hashSignaturePath     string
	formatSettings        verityFormatSettings
}

type verityFormatSettings struct {
	hashAlgorithm      string
	dataBlockSizeBytes uint32
	hashBlockSizeBytes uint32
	// The size of the data covered by the verity tree. The value should be a multiple of dataBlockSizeBytes.
	// A value of 0 means the entire partition is covered.
	dataSizeBytes uint64
	// If inline verity is used, the offset within the partition that contains the start of verity tree. Otherwise, 0.
	hashOffsetBytes uint64
}

func (s *verityFormatSettings) IsInlineVerity() bool {
	return s.hashOffsetBytes != 0
}
