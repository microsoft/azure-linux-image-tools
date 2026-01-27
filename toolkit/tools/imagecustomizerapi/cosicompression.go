// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import "fmt"

// CosiCompression specifies the compression settings for COSI output format.
type CosiCompression struct {
	// Level specifies the zstd compression level (1-22).
	// Higher levels provide better compression but take longer.
	// Levels 20-22 require additional memory (--ultra mode is automatically enabled).
	// Default: 9
	Level *int `yaml:"level" json:"level,omitempty"`
}

const (
	// MinZstdCompressionLevel is the minimum zstd compression level.
	MinZstdCompressionLevel = 1

	// MaxZstdCompressionLevel is the maximum zstd compression level.
	MaxZstdCompressionLevel = 22

	// UltraZstdCompressionThreshold is the level at which --ultra is required.
	UltraZstdCompressionThreshold = 20

	// DefaultCosiCompressionLevel is the default zstd compression level for cosi format.
	DefaultCosiCompressionLevel = 9

	// DefaultCosiCompressionLong is the zstd --long window size for cosi format (2^27 = 128 MiB).
	DefaultCosiCompressionLong = 27

	// DefaultBareMetalCompressionLevel is the default zstd compression level for baremetal-image format.
	DefaultBareMetalCompressionLevel = 22

	// DefaultBareMetalCompressionLong is the zstd --long window size for baremetal-image format (2^31 = 2 GiB).
	DefaultBareMetalCompressionLong = 31
)

func (c *CosiCompression) IsValid() error {
	if c.Level != nil && (*c.Level < MinZstdCompressionLevel || *c.Level > MaxZstdCompressionLevel) {
		return fmt.Errorf("invalid 'level' value (%d): must be between %d and %d",
			*c.Level, MinZstdCompressionLevel, MaxZstdCompressionLevel)
	}

	return nil
}
