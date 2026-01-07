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

			// Validate passthrough mode compatibility
			if c.OS.Uki.Mode == UkiModePassthrough {
				err = c.validateUkiPassthroughMode()
				if err != nil {
					return err
				}
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

	if c.Output.SelinuxPolicyPath != "" {
		if !sliceutils.ContainsValue(c.PreviewFeatures, PreviewFeatureOutputSelinuxPolicy) {
			return fmt.Errorf("the 'output-selinux-policy' preview feature must be enabled to use 'output.selinuxPolicyPath'")
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

	if c.Input.Image.Oci != nil &&
		!sliceutils.ContainsValue(c.PreviewFeatures, PreviewFeatureInputImageOci) {
		return fmt.Errorf("the '%s' preview feature must be enabled to use 'input.image.oci'",
			PreviewFeatureInputImageOci)
	}

	if c.Output.Image.Cosi.Compression.Level != nil &&
		!sliceutils.ContainsValue(c.PreviewFeatures, PreviewFeatureCosiCompression) {
		return fmt.Errorf("the '%s' preview feature must be enabled to use 'output.image.cosi.compression'",
			PreviewFeatureCosiCompression)
	}

	hasBtrfsFilesystem := slices.ContainsFunc(c.Storage.FileSystems, func(fs FileSystem) bool {
		return fs.Type == FileSystemTypeBtrfs
	})

	if hasBtrfsFilesystem && !sliceutils.ContainsValue(c.PreviewFeatures, PreviewFeatureBtrfs) {
		return fmt.Errorf("the '%s' preview feature must be enabled to use btrfs filesystems",
			PreviewFeatureBtrfs)
	}

	return nil
}

func (c *Config) CustomizePartitions() bool {
	return c.Storage.CustomizePartitions()
}

func (c *Config) validateUkiPassthroughMode() error {
	var incompatibleConfigs []string

	// Check for bootloader hard-reset (modifies bootloader configuration)
	if c.OS != nil && c.OS.BootLoader.ResetType == ResetBootLoaderTypeHard {
		incompatibleConfigs = append(incompatibleConfigs,
			"os.bootloader.resetType: hard-reset modifies bootloader configuration")
	}

	// Check for SELinux mode changes (modifies kernel command line)
	if c.OS != nil && c.OS.SELinux.Mode != SELinuxModeDefault {
		incompatibleConfigs = append(incompatibleConfigs,
			"os.selinux.mode: changes SELinux mode which modifies kernel command line embedded in UKI")
	}

	// Check for kernel command line modifications
	if c.OS != nil && len(c.OS.KernelCommandLine.ExtraCommandLine) > 0 {
		incompatibleConfigs = append(incompatibleConfigs,
			"os.kernelCommandLine.extraCommandLine: modifies kernel command line embedded in UKI")
	}

	// Check for verity configuration (modifies initramfs and kernel cmdline)
	if len(c.Storage.Verity) > 0 {
		incompatibleConfigs = append(incompatibleConfigs,
			"storage.verity: adds verity devices which modifies initramfs and kernel cmdline")
	}

	// Check for verity reinitialization (modifies initramfs and kernel cmdline)
	if c.Storage.ReinitializeVerity == ReinitializeVerityTypeAll {
		incompatibleConfigs = append(incompatibleConfigs,
			"storage.reinitializeVerity: reinitializes verity which modifies initramfs and kernel cmdline")
	}

	// Check for overlay configurations (might trigger initramfs regeneration)
	if c.OS != nil && c.OS.Overlays != nil && len(*c.OS.Overlays) > 0 {
		incompatibleConfigs = append(incompatibleConfigs,
			"os.overlays: overlay configuration might trigger initramfs regeneration")
	}

	if len(incompatibleConfigs) > 0 {
		errorMsg := "UKI passthrough mode is incompatible with the following configurations:\n"
		for _, cfg := range incompatibleConfigs {
			errorMsg += fmt.Sprintf("  - %s\n", cfg)
		}
		errorMsg += "\nPassthrough mode preserves existing UKIs without modification.\n"
		errorMsg += "To make these changes, use mode: create to regenerate UKIs, or remove the incompatible configurations."
		return fmt.Errorf("%s", errorMsg)
	}

	// Note: Kernel package modifications are validated at runtime by checking /boot
	// for new kernel binaries after package operations. This is more reliable than
	// checking package names statically.
	return nil
}
