// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
)

// From: https://gitlab.com/cryptsetup/cryptsetup/-/wikis/DMVerity
type veritySuperBlock struct {
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

func calculateHashFileSizeInBytes(hashPartitionPath string) (uint64, error) {
	hashPartition, err := os.Open(hashPartitionPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open hash partition (%s) block device:\n%w", hashPartitionPath, err)
	}
	defer hashPartition.Close()

	superblock := veritySuperBlock{}
	err = binary.Read(hashPartition, binary.LittleEndian, &superblock)
	if err != nil {
		return 0, fmt.Errorf("failed to read hash partition's (%s) superblock:\n%w", hashPartitionPath, err)
	}

	sizeInBytes, err := calculateHashFileSizeInBytesFromSuperBlock(superblock)
	if err != nil {
		return 0, fmt.Errorf("hash partition's (%s) superblock is invalid:\n%w", hashPartitionPath, err)
	}

	return sizeInBytes, nil
}

func calculateHashFileSizeInBytesFromSuperBlock(superblock veritySuperBlock) (uint64, error) {
	var err error

	if string(superblock.Signature[:]) != "verity\x00\x00" {
		return 0, fmt.Errorf("wrong superblock signature")
	}

	if superblock.Version != 1 {
		return 0, fmt.Errorf("unsupported version (%d)", superblock.Version)
	}

	if superblock.HashType != 1 {
		return 0, fmt.Errorf("unsupported hash type (%d)", superblock.HashType)
	}

	algorithmBytes, _, _ := bytes.Cut(superblock.Algorithm[:], []byte{0})
	algorithm := string(algorithmBytes)

	hashSize := uint32(0)
	switch algorithm {
	case "sha256":
		hashSize = 32

	case "sha384":
		hashSize = 48

	case "sha512":
		hashSize = 64

	default:
		return 0, fmt.Errorf("unknown hash algorithm (%s)", algorithm)
	}

	sizeInBytes, err := calculateHashFileSizeInBytesHelper(superblock.DataBlocks, superblock.DataBlockSize,
		superblock.HashBlockSize, hashSize)
	if err != nil {
		return 0, err
	}

	return sizeInBytes, nil
}

func calculateHashFileSizeInBytesHelper(dataBlocksCount uint64, dataBlockSize uint32, hashBlockSize uint32,
	hashSize uint32,
) (uint64, error) {
	if !isPowerOf2(dataBlockSize) {
		return 0, fmt.Errorf("invalid data block size (%d)", dataBlockSize)
	}

	if !isPowerOf2(hashBlockSize) || hashBlockSize < hashSize {
		return 0, fmt.Errorf("invalid hash block size (%d)", hashBlockSize)
	}

	// dm-verity pads each hash to the nearest power-of-2 to make the math easier.
	hashSizeFull := roundUpToPowerOf2(hashSize)

	hashesPerBlock := uint64(hashBlockSize / hashSizeFull)

	totalTreeBlocks := uint64(0)
	prevLevelTreeBlocks := dataBlocksCount
	for prevLevelTreeBlocks > 1 {
		levelTreeBlocks := prevLevelTreeBlocks / hashesPerBlock
		rem := prevLevelTreeBlocks % hashesPerBlock
		if rem != 0 {
			// Round up the nearest whole block.
			levelTreeBlocks += 1
		}

		totalTreeBlocks += levelTreeBlocks
		prevLevelTreeBlocks = levelTreeBlocks
	}

	totalBlocks := totalTreeBlocks + 1 // add superblock
	totalBytes := totalBlocks * uint64(hashBlockSize)
	return totalBytes, nil
}

func isPowerOf2(n uint32) bool {
	return (n & (n - 1)) == 0
}

func roundUpToPowerOf2(n uint32) uint32 {
	res := uint32(1)
	for res < n {
		res *= 2
	}
	return res
}
