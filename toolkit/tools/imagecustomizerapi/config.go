// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type Config struct {
	Storage         Storage  `yaml:"storage"`
	Iso             *Iso     `yaml:"iso"`
	Pxe             *Pxe     `yaml:"pxe"`
	OS              *OS      `yaml:"os"`
	Scripts         Scripts  `yaml:"scripts"`
	PreviewFeatures []string `yaml:"previewFeatures"`
}

func (c *Config) IsValid() (err error) {
	err = c.Storage.IsValid()
	if err != nil {
		return err
	}

	hasResetPartitionsUuids := c.Storage.ResetPartitionsUuidsType != ResetPartitionsUuidsTypeDefault

	if c.Iso != nil {
		err = c.Iso.IsValid()
		if err != nil {
			return fmt.Errorf("invalid 'iso' field:\n%w", err)
		}
	}

	if c.Pxe != nil {
		err = c.Pxe.IsValid()
		if err != nil {
			return fmt.Errorf("invalid 'pxe' field:\n%w", err)
		}
	}

	hasResetBootLoader := false
	if c.OS != nil {
		err = c.OS.IsValid()
		if err != nil {
			return fmt.Errorf("invalid 'os' field:\n%w", err)
		}
		hasResetBootLoader = c.OS.BootLoader.ResetType != ResetBootLoaderTypeDefault

		if c.OS.Uki != nil {
			// Ensure "uki" is included in PreviewFeatures at this time.
			if !containsPreviewFeature(c.PreviewFeatures, "uki") {
				return fmt.Errorf("the 'uki' preview feature must be enabled to use 'os.uki'")
			}

			// Temporary limitation: We currently require 'os.bootloader.reset' to be 'hard-reset' when 'os.uki' is enabled.
			// In the future, as we design and develop the bootloader further, this hard-reset limitation may be lifted.
			if c.OS.BootLoader.ResetType != ResetBootLoaderTypeHard {
				return fmt.Errorf(
					"'os.bootloader.reset' must be '%s' when 'os.uki' is enabled", ResetBootLoaderTypeHard,
				)
			}
		}
	}

	err = c.Scripts.IsValid()
	if err != nil {
		return err
	}

	if c.CustomizePartitions() && !hasResetBootLoader {
		return fmt.Errorf("'os.bootloader.reset' must be specified if 'storage.disks' is specified")
	}

	if hasResetPartitionsUuids && !hasResetBootLoader {
		return fmt.Errorf("'os.bootloader.reset' must be specified if 'storage.resetPartitionsUuidsType' is specified")
	}

	return nil
}

func (c *Config) CustomizePartitions() bool {
	return c.Storage.CustomizePartitions()
}

func containsPreviewFeature(features []string, feature string) bool {
	for _, f := range features {
		if f == feature {
			return true
		}
	}
	return false
}
