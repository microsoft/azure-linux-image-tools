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
	dataSizeBytes      uint64
	hashOffsetBytes    uint64
}

func (s *verityFormatSettings) IsInlineVerity() bool {
	return s.hashOffsetBytes != 0
}
