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

func calculateHashFileSystemSize(hashPartitionPath string) (uint64, error) {
	hashPartition, err := os.Open(hashPartitionPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open hash partition (%s) block device:\n%w", hashPartitionPath, err)
	}
	defer hashPartition.Close()

	superblock := veritySuperBlock{}
	err = binary.Read(hashPartition, binary.LittleEndian, &superblock)
	if err != nil {
		return 0, fmt.Errorf("failed to read hash partition's (%s) superblock", hashPartitionPath)
	}

	if string(superblock.Signature[:]) != "verity\x00\x00" {
		return 0, fmt.Errorf("hash partition's (%s) superblock has wrong signature", hashPartitionPath)
	}

	if superblock.Version != 1 {
		return 0, fmt.Errorf("hash partition's (%s) superblock has unsupported version (%d)", hashPartitionPath,
			superblock.Version)
	}

	if superblock.HashType != 1 {
		return 0, fmt.Errorf("hash partition's (%s) superblock has unsupported hash type (%d)", hashPartitionPath,
			superblock.HashType)
	}

	algorithmBytes, _, _ := bytes.Cut(superblock.Algorithm[:], []byte{0})
	algorithm := string(algorithmBytes)

	hashSize := uint32(0)
	switch algorithm {
	case "sha256":
		hashSize = 32

	default:
		return 0, fmt.Errorf("hash partition's (%s) superblock has unknown algorithm (%s):\n%w", hashPartitionPath,
			algorithm, err)
	}

	if !isPowerOf2(superblock.DataBlockSize) {
		return 0, fmt.Errorf("hash partition's (%s) superblock has unsupported data block size (%d):\n%w",
			hashPartitionPath, superblock.DataBlockSize, err)
	}

	if !isPowerOf2(superblock.HashBlockSize) || superblock.HashBlockSize < hashSize {
		return 0, fmt.Errorf("hash partition's (%s) superblock has unsupported hash block size (%d):\n%w",
			hashPartitionPath, superblock.HashBlockSize, err)
	}

	hashesPerBlock := uint64(superblock.HashBlockSize / hashSize)

	totalTreeBlocks := uint64(0)
	prevLevelTreeBlocks := superblock.DataBlocks
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
	totalBytes := totalBlocks * uint64(superblock.HashBlockSize)

	return totalBytes, nil
}

func isPowerOf2(n uint32) bool {
	return (n & (n - 1)) == 0
}
