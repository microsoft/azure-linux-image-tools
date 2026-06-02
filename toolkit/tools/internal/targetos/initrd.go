// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package targetos

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/cavaliergopher/cpio"
	"github.com/klauspost/compress/zstd"
	"github.com/klauspost/pgzip"
)

// Magic byte signatures for the compression formats we recognize at the start
// of an initramfs stream.
//
//   - gzip: RFC 1952 section 2.3.1 ("ID1 ID2" = 1F 8B)
//   - xz:   xz file-format specification section 2.1.1.1 ("Header Magic Bytes")
//   - zstd: RFC 8478 section 3.1.1 ("Magic_Number" = 0xFD2FB528 little-endian)
var (
	magicGzip        = []byte{0x1f, 0x8b}
	magicXz          = []byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00}
	magicZstd        = []byte{0x28, 0xb5, 0x2f, 0xfd}
	longestMagicSize = len(magicXz)
)

// readFirstFileFromInitrd scans an initramfs cpio archive once and returns the content of the first candidate path in
// the provided list that exists as a regular file. Returns an error if none of the candidates is present.
func readFirstFileFromInitrd(initrdPath string, candidates []string) (content []byte, foundPath string, err error) {
	f, err := os.Open(initrdPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open initrd (%s):\n%w", initrdPath, err)
	}
	defer f.Close()

	decompressed, err := openInitrdDecompressor(f)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read initrd (%s):\n%w", initrdPath, err)
	}
	defer decompressed.Close()

	wanted := make(map[string]struct{}, len(candidates))
	for _, c := range candidates {
		wanted[c] = struct{}{}
	}

	captured := make(map[string][]byte, len(candidates))
	cpioReader := cpio.NewReader(decompressed)
	for {
		hdr, err := cpioReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", fmt.Errorf("failed to read cpio header from initrd (%s):\n%w", initrdPath, err)
		}

		if _, ok := wanted[hdr.Name]; !ok {
			continue
		}

		// Only capture regular files.
		if hdr.Mode&cpio.ModeType != cpio.TypeReg {
			continue
		}

		data, err := io.ReadAll(cpioReader)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read (%s) from initrd (%s):\n%w", hdr.Name, initrdPath, err)
		}

		captured[hdr.Name] = data
	}

	for _, candidate := range candidates {
		if data, ok := captured[candidate]; ok {
			return data, candidate, nil
		}
	}

	return nil, "", fmt.Errorf("failed to find any of %v in initrd (%s)", candidates, initrdPath)
}

// openInitrdDecompressor auto-detects the compression format of an initramfs stream from its leading magic bytes and
// returns a reader over the decompressed (cpio) content.
func openInitrdDecompressor(r io.Reader) (io.ReadCloser, error) {
	br := bufio.NewReader(r)
	head, err := br.Peek(longestMagicSize)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to peek initrd magic bytes:\n%w", err)
	}

	switch {
	case bytes.HasPrefix(head, magicGzip):
		gz, err := pgzip.NewReader(br)
		if err != nil {
			return nil, fmt.Errorf("failed to open gzip reader for initrd:\n%w", err)
		}
		return gz, nil

	case bytes.HasPrefix(head, magicXz):
		return nil, fmt.Errorf("xz-compressed initrd is not yet supported")

	case bytes.HasPrefix(head, magicZstd):
		zr, err := zstd.NewReader(br)
		if err != nil {
			return nil, fmt.Errorf("failed to open zstd reader for initrd:\n%w", err)
		}

		// zstd.Decoder satisfies io.Reader but not io.Closer since its Close() returns no error, so wrap it to
		// implement io.ReadCloser, like pgzip.Reader.
		return zstdReadCloser{Decoder: zr}, nil

	default:
		return nil, fmt.Errorf("unrecognized initrd compression format (leading bytes: % x)", head)
	}
}
