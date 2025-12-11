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
	Level int `yaml:"level" json:"level,omitempty"`
}

const (
	// DefaultCosiCompressionLevel is the default zstd compression level.
	DefaultCosiCompressionLevel = 9

	// MinCosiCompressionLevel is the minimum zstd compression level.
	MinCosiCompressionLevel = 1

	// MaxCosiCompressionLevel is the maximum zstd compression level.
	MaxCosiCompressionLevel = 22

	// UltraCosiCompressionThreshold is the level at which --ultra is required.
	UltraCosiCompressionThreshold = 20

	// CosiLongWindowSize is the zstd --long window size for COSI format (2^27 = 128 MiB).
	CosiLongWindowSize = 27
)

func (c *CosiCompression) IsValid() error {
	if c.Level != 0 && (c.Level < MinCosiCompressionLevel || c.Level > MaxCosiCompressionLevel) {
		return fmt.Errorf("invalid 'level' value (%d): must be between %d and %d",
			c.Level, MinCosiCompressionLevel, MaxCosiCompressionLevel)
	}

	return nil
}

// GetLevel returns the compression level to use, defaulting to DefaultCosiCompressionLevel if not set.
func (c *CosiCompression) GetLevel() int {
	if c.Level == 0 {
		return DefaultCosiCompressionLevel
	}

	return c.Level
}
