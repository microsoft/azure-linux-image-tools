// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"errors"
	"fmt"
)

// ErrInvalidCosiCompressionLevelArg is returned when an invalid --cosi-compression-level CLI argument is provided.
var ErrInvalidCosiCompressionLevelArg = errors.New("invalid --cosi-compression-level value")

// CosiCompression specifies the compression settings for COSI output format.
type CosiCompression struct {
	// Level specifies the zstd compression level (1-22).
	// Higher levels provide better compression but take longer.
	// Levels 20-22 require additional memory (--ultra mode is automatically enabled).
	// Default: 9
	Level *int `yaml:"level" json:"level,omitempty"`
}

const (
	// MinCosiCompressionLevel is the minimum zstd compression level.
	MinCosiCompressionLevel = 1

	// MaxCosiCompressionLevel is the maximum zstd compression level.
	MaxCosiCompressionLevel = 22

	// UltraCosiCompressionThreshold is the level at which --ultra is required.
	UltraCosiCompressionThreshold = 20

	// DefaultBareMetalCosiCompressionLevel is the default zstd compression level for baremetal-image format.
	DefaultBareMetalCosiCompressionLevel = 22

	// DefaultBareMetalCosiCompressionLong is the default zstd --long window size for baremetal-image format (2^31 = 2 GiB).
	DefaultBareMetalCosiCompressionLong = 31

	// DefaultCosiCompressionLevel is the default zstd compression level for other formats.
	DefaultCosiCompressionLevel = 9

	// DefaultCosiCompressionLong is the default zstd --long window size (2^27 = 128 MiB) for other formats.
	DefaultCosiCompressionLong = 27
)

func (c *CosiCompression) IsValid() error {
	if c.Level != nil && (*c.Level < MinCosiCompressionLevel || *c.Level > MaxCosiCompressionLevel) {
		return fmt.Errorf("invalid 'level' value (%d): must be between %d and %d",
			*c.Level, MinCosiCompressionLevel, MaxCosiCompressionLevel)
	}

	return nil
}

// ValidateCosiCompressionLevel validates a COSI compression level CLI argument.
func ValidateCosiCompressionLevel(level *int) error {
	if level != nil &&
		(*level < MinCosiCompressionLevel || *level > MaxCosiCompressionLevel) {
		return fmt.Errorf("%w (level=%d, valid range: %d-%d)",
			ErrInvalidCosiCompressionLevelArg, *level,
			MinCosiCompressionLevel, MaxCosiCompressionLevel)
	}
	return nil
}
