// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
	"slices"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/sliceutils"
)

type Config struct {
	Input           Input            `yaml:"input" json:"input,omitempty"`
	Storage         Storage          `yaml:"storage" json:"storage,omitempty"`
	Iso             *Iso             `yaml:"iso" json:"iso,omitempty"`
	Pxe             *Pxe             `yaml:"pxe" json:"pxe,omitempty"`
	OS              *OS              `yaml:"os" json:"os,omitempty"`
	Scripts         Scripts          `yaml:"scripts" json:"scripts,omitempty"`
	PreviewFeatures []PreviewFeature `yaml:"previewFeatures" json:"previewFeatures,omitempty"`
	Output          Output           `yaml:"output" json:"output,omitempty"`
	BaseConfigs     []BaseConfig     `yaml:"baseConfigs" json:"baseConfigs,omitempty"`
}

func (c *Config) IsValid() (err error) {
	err = c.Input.IsValid()
	if err != nil {
		return fmt.Errorf("invalid 'input' field:\n%w", err)
	}

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
		if c.Iso.KdumpBootFiles != nil && !sliceutils.ContainsValue(c.PreviewFeatures, PreviewFeatureKdumpBootFiles) {
			return fmt.Errorf("the '%s' preview feature must be enabled to use 'iso.kdumpBootFiles'", PreviewFeatureKdumpBootFiles)
		}
	}

	if c.Pxe != nil {
		err = c.Pxe.IsValid()
		if err != nil {
			return fmt.Errorf("invalid 'pxe' field:\n%w", err)
		}
		if c.Pxe.KdumpBootFiles != nil && !sliceutils.ContainsValue(c.PreviewFeatures, PreviewFeatureKdumpBootFiles) {
			return fmt.Errorf("the '%s' preview feature must be enabled to use 'pxe.kdumpBootFiles'", PreviewFeatureKdumpBootFiles)
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
			if !sliceutils.ContainsValue(c.PreviewFeatures, PreviewFeatureUki) {
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

		if c.OS.Packages.SnapshotTime != "" && !sliceutils.ContainsValue(c.PreviewFeatures, PreviewFeaturePackageSnapshotTime) {
			return fmt.Errorf("the 'package-snapshot-time' preview feature must be enabled to use 'os.packages.snapshotTime'")
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

	err = c.Output.IsValid()
	if err != nil {
		return fmt.Errorf("invalid 'output' field:\n%w", err)
	}

	if c.Output.Artifacts != nil {
		// Ensure "outputArtifacts" is included in PreviewFeatures at this time.
		if !sliceutils.ContainsValue(c.PreviewFeatures, PreviewFeatureOutputArtifacts) {
			return fmt.Errorf("the 'output-artifacts' preview feature must be enabled to use 'output.artifacts'")
		}
	}

	// Check if any verity entry has a non-empty hash signature path.
	hasVerityHashSignature := slices.ContainsFunc(c.Storage.Verity, func(v Verity) bool {
		return v.HashSignaturePath != ""
	})

	if hasVerityHashSignature {
		if !sliceutils.ContainsValue(c.PreviewFeatures, PreviewFeatureOutputArtifacts) {
			return fmt.Errorf("the 'output-artifacts' preview feature must be enabled to use 'verity.hashSignaturePath'")
		}
	}

	if c.Storage.ReinitializeVerity != ReinitializeVerityTypeDefault &&
		!sliceutils.ContainsValue(c.PreviewFeatures, PreviewFeatureReinitializeVerity) {
		return fmt.Errorf("the 'reinitialize-verity' preview feature must be enabled to use 'storage.reinitializeVerity'")
	}

	if c.BaseConfigs != nil {
		if !sliceutils.ContainsValue(c.PreviewFeatures, PreviewFeatureBaseConfigs) {
			return fmt.Errorf("the '%s' preview feature must be enabled to use 'baseConfigs'", PreviewFeatureBaseConfigs)
		}

		for i, base := range c.BaseConfigs {
			if err := base.IsValid(); err != nil {
				return fmt.Errorf("invalid baseConfig item at index %d:\n%w", i, err)
			}
		}
	}

	return nil
}

func (c *Config) CustomizePartitions() bool {
	return c.Storage.CustomizePartitions()
}
