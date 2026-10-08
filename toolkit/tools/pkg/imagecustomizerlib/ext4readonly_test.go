// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestReadExtFilesystemReadOnly(t *testing.T) {
	for _, tc := range []struct {
		name     string
		features uint32
		readonly bool
	}{
		{"writable", 0, false},
		{"metadata-checksums", 0x400, false},
		{"readonly", 0x1000, true},
		{"readonly-with-checksums", 0x1400, true},
		{"legacy-high-byte", 0xff000000, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := make([]byte, 2048)
			binary.LittleEndian.PutUint16(data[1024+56:], 0xef53)
			binary.LittleEndian.PutUint32(data[1024+100:], tc.features)
			before := bytes.Clone(data)
			got, err := readExtFilesystemReadOnly(bytes.NewReader(data))
			if err != nil || got != tc.readonly {
				t.Fatalf("got (%v, %v), want (%v, nil)", got, err, tc.readonly)
			}
			if !bytes.Equal(before, data) {
				t.Fatal("reading features modified the filesystem")
			}
		})
	}
}

func TestReadExtFilesystemReadOnlyRejectsInvalidInput(t *testing.T) {
	for _, size := range []int{0, 1024, 1127, 2048} {
		if _, err := readExtFilesystemReadOnly(bytes.NewReader(make([]byte, size))); err == nil {
			t.Fatalf("accepted truncated or invalid superblock of size %d", size)
		}
	}
}
