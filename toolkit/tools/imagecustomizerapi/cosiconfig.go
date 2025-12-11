// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import "fmt"

// CosiConfig specifies the configuration options for COSI output format.
type CosiConfig struct {
	// Compression specifies the compression settings for COSI partition images.
	Compression *CosiCompression `yaml:"compression" json:"compression,omitempty"`
}

func (c *CosiConfig) IsValid() error {
	if c.Compression != nil {
		if err := c.Compression.IsValid(); err != nil {
			return fmt.Errorf("invalid 'compression' value:\n%w", err)
		}
	}

	return nil
}
