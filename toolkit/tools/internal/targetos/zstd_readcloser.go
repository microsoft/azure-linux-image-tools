// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package targetos

import (
	"github.com/klauspost/compress/zstd"
)

// zstdReadCloser adapts *zstd.Decoder (whose Close() returns no error) to io.ReadCloser.
type zstdReadCloser struct{ *zstd.Decoder }

func (z zstdReadCloser) Close() error {
	z.Decoder.Close()
	return nil
}
