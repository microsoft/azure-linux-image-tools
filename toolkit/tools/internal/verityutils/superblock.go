// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package verityutils

import (
	"bytes"
)

// From: https://gitlab.com/cryptsetup/cryptsetup/-/wikis/DMVerity
type VeritySuperBlock struct {
	Signature     [8]uint8   // "verity\0\0"
	Version       uint32     // Superblock version: 1
	HashType      uint32     // 0: Chrome OS, 1: normal
	Uuid          [16]uint8  // UUID of hash device
	Algorithm     [32]uint8  // Hash algorithm name
	DataBlockSize uint32     // Data block in bytes
	HashBlockSize uint32     // Hash block in bytes
	DataBlocks    uint64     // Number of data blocks
	SaltSize      uint16     // Salt size
	Pad1          [6]uint8   // Padding
	Salt          [256]uint8 // Salt
	Pad2          [168]uint8 // Padding
}

func (s *VeritySuperBlock) GetAlgorithm() string {
	algorithmBytes, _, _ := bytes.Cut(s.Algorithm[:], []byte{0})
	algorithm := string(algorithmBytes)
	return algorithm
}
