// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package verityutils

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

func CalculateHashFileSizeInBytes(hashPartitionPath string, hashOffsetBytes uint64) (uint64, error) {
	superblock, err := ReadVeritySuperblock(hashPartitionPath, hashOffsetBytes)
	if err != nil {
		return 0, err
	}

	sizeInBytes, err := CalculateHashSizeInBytes(superblock.DataBlocks, superblock.HashBlockSize,
		superblock.GetAlgorithm())
	if err != nil {
		return 0, err
	}

	return sizeInBytes, nil
}

func ReadVeritySuperblock(hashPartitionPath string, hashOffsetBytes uint64) (VeritySuperBlock, error) {
	hashPartition, err := os.Open(hashPartitionPath)
	if err != nil {
		return VeritySuperBlock{}, fmt.Errorf("failed to open hash partition (%s) block device:\n%w", hashPartitionPath, err)
	}
	defer hashPartition.Close()

	if hashOffsetBytes != 0 {
		_, err := hashPartition.Seek(int64(hashOffsetBytes), io.SeekStart)
		if err != nil {
			return VeritySuperBlock{}, fmt.Errorf("failed to seek to hash partition's (%s) superblock:\n%w", hashPartitionPath, err)
		}
	}

	superblock := VeritySuperBlock{}
	err = binary.Read(hashPartition, binary.LittleEndian, &superblock)
	if err != nil {
		return VeritySuperBlock{}, fmt.Errorf("failed to read hash partition's (%s) superblock:\n%w", hashPartitionPath, err)
	}

	err = verifySuperblock(superblock)
	if err != nil {
		return VeritySuperBlock{}, err
	}

	return superblock, nil
}

func verifySuperblock(superblock VeritySuperBlock) error {
	if string(superblock.Signature[:]) != "verity\x00\x00" {
		return fmt.Errorf("wrong superblock signature")
	}

	if superblock.Version != 1 {
		return fmt.Errorf("unsupported version (%d)", superblock.Version)
	}

	if superblock.HashType != 1 {
		return fmt.Errorf("unsupported hash type (%d)", superblock.HashType)
	}

	if !isPowerOf2(superblock.DataBlockSize) {
		return fmt.Errorf("invalid data block size (%d)", superblock.DataBlockSize)
	}

	_, err := verifyHashAlgorithmAndBlockSize(superblock.GetAlgorithm(), superblock.HashBlockSize)
	if err != nil {
		return err
	}

	return nil
}

func verifyHashAlgorithmAndBlockSize(algorithm string, hashBlockSize uint32) (uint32, error) {
	hashSize, err := getAlgorithmHashSize(algorithm)
	if err != nil {
		return 0, err
	}

	if !isPowerOf2(hashBlockSize) || hashBlockSize < hashSize {
		return 0, fmt.Errorf("invalid hash block size (%d)", hashBlockSize)
	}

	return hashSize, nil
}

func getAlgorithmHashSize(algorithm string) (uint32, error) {
	switch algorithm {
	case "sha256":
		return 32, nil

	case "sha384":
		return 48, nil

	case "sha512":
		return 64, nil

	default:
		return 0, fmt.Errorf("unknown hash algorithm (%s)", algorithm)
	}
}

func CalculateHashSizeInBytes(dataBlocksCount uint64, hashBlockSize uint32, algorithm string) (uint64, error) {
	totalBlocks, err := CalculateHashSizeInBlocks(dataBlocksCount, hashBlockSize, algorithm)
	if err != nil {
		return 0, err
	}

	totalBytes := totalBlocks * uint64(hashBlockSize)
	return totalBytes, nil
}

func CalculateHashSizeInBlocks(dataBlocksCount uint64, hashBlockSize uint32, algorithm string) (uint64, error) {
	hashSize, err := verifyHashAlgorithmAndBlockSize(algorithm, hashBlockSize)
	if err != nil {
		return 0, err
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
	return totalBlocks, nil
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
