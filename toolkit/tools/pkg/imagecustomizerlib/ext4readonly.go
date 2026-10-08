// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

func extFilesystemReadOnly(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("failed to open ext filesystem (%s):\n%w", path, err)
	}
	defer f.Close()
	return readExtFilesystemReadOnly(f)
}

func readExtFilesystemReadOnly(r io.ReaderAt) (bool, error) {
	var superblock [104]byte
	if _, err := r.ReadAt(superblock[:], 1024); err != nil {
		return false, fmt.Errorf("failed to read ext superblock:\n%w", err)
	}
	if binary.LittleEndian.Uint16(superblock[56:58]) != 0xef53 {
		return false, fmt.Errorf("invalid ext superblock magic")
	}
	return binary.LittleEndian.Uint32(superblock[100:104])&0x1000 != 0, nil
}
