// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/ptrutils"
	"github.com/stretchr/testify/assert"
)

func TestConfigIsValid(t *testing.T) {
	config := &Config{
		Input: Input{
			Image: InputImage{
				Path: "./base.vhdx",
			},
		},
		Storage: Storage{
			Disks: []Disk{{
				PartitionTableType: "gpt",
				MaxSize:            ptrutils.PtrTo(DiskSize(3 * diskutils.MiB)),
				Partitions: []Partition{
					{
						Id:    "esp",
						Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
						Type:  PartitionTypeESP,
					},
				},
			}},
			BootType: "efi",
			FileSystems: []FileSystem{
				{
					DeviceId: "esp",
					Type:     "fat32",
					MountPoint: &MountPoint{
						Path: "/boot/efi",
					},
				},
			},
		},
		OS: &OS{
			BootLoader: BootLoader{
				ResetType: "hard-reset",
			},
			Hostname: "test",
		},
		Scripts: Scripts{},
		Iso:     &Iso{},
		Output: Output{
			Image: OutputImage{
				Path:   "./out/image.vhdx",
				Format: ImageFormatTypeVhdx,
			},
		},
	}

	err := config.IsValid()
	assert.NoError(t, err)
}

func TestConfigIsValidLegacy(t *testing.T) {
	config := &Config{
		Storage: Storage{
			Disks: []Disk{{
				PartitionTableType: "gpt",
				MaxSize:            ptrutils.PtrTo(DiskSize(3 * diskutils.MiB)),
				Partitions: []Partition{
					{
						Id:    "boot",
						Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
						Type:  PartitionTypeBiosGrub,
					},
				},
			}},
			BootType: "legacy",
			FileSystems: []FileSystem{
				{
					DeviceId: "boot",
				},
			},
		},
		OS: &OS{
			BootLoader: BootLoader{
				ResetType: "hard-reset",
			},
			Hostname: "test",
		},
	}

	err := config.IsValid()
	assert.NoError(t, err)
}

func TestConfigIsValidNoBootType(t *testing.T) {
	config := &Config{
		Storage: Storage{
			Disks: []Disk{{
				PartitionTableType: "gpt",
				MaxSize:            ptrutils.PtrTo(DiskSize(3 * diskutils.MiB)),
				Partitions: []Partition{
					{
						Id:    "a",
						Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
					},
				},
			}},
		},
		OS: &OS{
			Hostname: "test",
			BootLoader: BootLoader{
				ResetType: "hard-reset",
			},
		},
	}

	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "must specify 'bootType' if 'disks' are specified")
}

func TestConfigIsValidMissingBootLoaderReset(t *testing.T) {
	config := &Config{
		Storage: Storage{
			Disks: []Disk{{
				PartitionTableType: "gpt",
				MaxSize:            ptrutils.PtrTo(DiskSize(3 * diskutils.MiB)),
				Partitions: []Partition{
					{
						Id:    "esp",
						Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
						Type:  PartitionTypeESP,
					},
				},
			}},
			BootType: "efi",
			FileSystems: []FileSystem{
				{
					DeviceId: "esp",
					Type:     "fat32",
					MountPoint: &MountPoint{
						Path: "/boot/efi",
					},
				},
			},
		},
		OS: &OS{
			Hostname: "test",
		},
	}

	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "'os.bootloader.reset' must be specified if 'storage.disks' is specified")
}

func TestConfigIsValidWithPreviewFeaturesAndUki(t *testing.T) {
	config := &Config{
		OS: &OS{
			BootLoader: BootLoader{
				ResetType: "hard-reset",
			},
			Uki: &Uki{
				Mode: UkiModeCreate,
			},
		},
		PreviewFeatures: []PreviewFeature{"uki"},
	}

	err := config.IsValid()
	assert.NoError(t, err)
}

func TestConfigIsValidWithMissingUkiPreviewFeature(t *testing.T) {
	config := &Config{
		OS: &OS{
			BootLoader: BootLoader{
				ResetType: "hard-reset",
			},
			Uki: &Uki{
				Mode: UkiModeCreate,
			},
		},
		PreviewFeatures: []PreviewFeature{},
	}

	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "the 'uki' preview feature must be enabled to use 'os.uki'")
}

func TestConfigIsValidWithInvalidBootType(t *testing.T) {
	config := &Config{
		Storage: Storage{
			BootType: "invalid-boot-type",
		},
		OS: &OS{
			BootLoader: BootLoader{
				ResetType: "hard-reset",
			},
		},
	}

	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid bootType value (invalid-boot-type)")
}

func TestConfigIsValidResetUuidsMissingBootLoaderReset(t *testing.T) {
	config := &Config{
		Storage: Storage{
			ResetPartitionsUuidsType: ResetPartitionsUuidsTypeAll,
		},
		OS: &OS{
			Hostname: "test",
		},
	}

	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "'os.bootloader.reset' must be specified if 'storage.resetPartitionsUuidsType' is specified")
}

func TestConfigIsValidMultipleDisks(t *testing.T) {
	config := &Config{
		Storage: Storage{
			Disks: []Disk{
				{
					PartitionTableType: "gpt",
					MaxSize:            ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
				},
				{
					PartitionTableType: "gpt",
					MaxSize:            ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
				},
			},
			BootType: "legacy",
		},
		OS: &OS{
			BootLoader: BootLoader{
				ResetType: "hard-reset",
			},
			Hostname: "test",
		},
	}

	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "defining multiple disks is not currently supported")
}

func TestConfigIsValidZeroDisks(t *testing.T) {
	config := &Config{
		Storage: Storage{
			BootType: BootTypeEfi,
			Disks:    []Disk{},
		},
		OS: &OS{
			Hostname: "test",
		},
	}

	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "cannot specify 'bootType' without specifying 'disks'")
}

func TestConfigIsValidBadHostname(t *testing.T) {
	config := &Config{
		OS: &OS{
			Hostname: "test_",
		},
	}

	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid hostname")
}

func TestConfigIsValidBadDisk(t *testing.T) {
	config := &Config{
		Storage: Storage{
			BootType: BootTypeEfi,
			Disks: []Disk{{
				PartitionTableType: PartitionTableTypeGpt,
				MaxSize:            ptrutils.PtrTo(DiskSize(0)),
			}},
		},
		OS: &OS{
			Hostname: "test",
		},
	}

	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid disk at index 0")
	assert.ErrorContains(t, err, "a disk's maxSize value (0) must be a positive non-zero number")
}

func TestConfigIsValidMissingEsp(t *testing.T) {
	config := &Config{
		Storage: Storage{
			Disks: []Disk{{
				PartitionTableType: "gpt",
				MaxSize:            ptrutils.PtrTo(DiskSize(2 * diskutils.MiB)),
				Partitions:         []Partition{},
			}},
			BootType: "efi",
		},
		OS: &OS{
			BootLoader: BootLoader{
				ResetType: "hard-reset",
			},
			Hostname: "test",
		},
	}

	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "'esp' partition must be provided for 'efi' boot type")
}

func TestConfigIsValidMissingBiosBoot(t *testing.T) {
	config := &Config{
		Storage: Storage{
			Disks: []Disk{{
				PartitionTableType: "gpt",
				MaxSize:            ptrutils.PtrTo(DiskSize(2 * diskutils.MiB)),
				Partitions:         []Partition{},
			}},
			BootType: "legacy",
		},
		OS: &OS{
			BootLoader: BootLoader{
				ResetType: "hard-reset",
			},
			Hostname: "test",
		},
	}

	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "'bios-grub' partition must be provided for 'legacy' boot type")
}

func TestConfigIsValidInvalidMountPoint(t *testing.T) {
	config := &Config{
		Storage: Storage{
			Disks: []Disk{{
				PartitionTableType: "gpt",
				MaxSize:            ptrutils.PtrTo(DiskSize(3 * diskutils.MiB)),
				Partitions: []Partition{
					{
						Id:    "esp",
						Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
						Type:  PartitionTypeESP,
					},
				},
			}},
			BootType: "efi",
			FileSystems: []FileSystem{
				{
					DeviceId: "esp",
					Type:     "fat32",
					MountPoint: &MountPoint{
						Path: "boot/efi",
					},
				},
			},
		},
		OS: &OS{
			BootLoader: BootLoader{
				ResetType: "hard-reset",
			},
			Hostname: "test",
		},
	}

	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid filesystems item at index 0")
	assert.ErrorContains(t, err, "invalid 'mountPoint' value")
	assert.ErrorContains(t, err, "invalid path (boot/efi): must be an absolute path")
}

func TestConfigIsValidKernelCLI(t *testing.T) {
	config := &Config{
		Storage: Storage{
			Disks: []Disk{{
				PartitionTableType: "gpt",
				MaxSize:            ptrutils.PtrTo(DiskSize(3 * diskutils.MiB)),
				Partitions: []Partition{
					{
						Id:    "esp",
						Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
						Type:  PartitionTypeESP,
					},
				},
			}},
			BootType: "efi",
			FileSystems: []FileSystem{
				{
					DeviceId: "esp",
					Type:     "fat32",
					MountPoint: &MountPoint{
						Path: "/boot/efi",
					},
				},
			},
		},
		OS: &OS{
			BootLoader: BootLoader{
				ResetType: "hard-reset",
			},
			Hostname: "test",

			KernelCommandLine: KernelCommandLine{
				ExtraCommandLine: []string{
					"console=ttyS0",
				},
			},
		},
	}
	err := config.IsValid()
	assert.NoError(t, err)
}

func TestConfigIsValidInvalidIso(t *testing.T) {
	config := &Config{
		Iso: &Iso{
			AdditionalFiles: AdditionalFileList{
				{},
			},
		},
	}
	err := config.IsValid()
	assert.ErrorContains(t, err, "invalid 'iso' field")
	assert.ErrorContains(t, err, "invalid additionalFiles")
}

func TestConfigIsValidInvalidScripts(t *testing.T) {
	config := &Config{
		Scripts: Scripts{
			PostCustomization: []Script{
				{
					Path: "",
				},
			},
		},
	}
	err := config.IsValid()
	assert.ErrorContains(t, err, "invalid postCustomization script at index 0")
	assert.ErrorContains(t, err, "either path or content must have a value")
}

func TestConfigIsValidVerityValid(t *testing.T) {
	config := &Config{
		Storage: Storage{
			Disks: []Disk{{
				PartitionTableType: "gpt",
				Partitions: []Partition{
					{
						Id: "esp",
						Size: PartitionSize{
							Type: PartitionSizeTypeExplicit,
							Size: 8 * diskutils.MiB,
						},
						Type: PartitionTypeESP,
					},
					{
						Id: "root",
						Size: PartitionSize{
							Type: PartitionSizeTypeExplicit,
							Size: 1 * diskutils.GiB,
						},
					},
					{
						Id: "verityhash",
						Size: PartitionSize{
							Type: PartitionSizeTypeExplicit,
							Size: 100 * diskutils.MiB,
						},
					},
				},
			}},
			BootType: "efi",
			FileSystems: []FileSystem{
				{
					DeviceId: "esp",
					Type:     "fat32",
					MountPoint: &MountPoint{
						Path: "/boot/efi",
					},
				},
				{
					DeviceId: "rootverity",
					Type:     "ext4",
					MountPoint: &MountPoint{
						Path:    "/",
						Options: "ro",
					},
				},
			},
			Verity: []Verity{
				{
					Id:           "rootverity",
					Name:         "root",
					DataDeviceId: "root",
					HashDeviceId: "verityhash",
				},
			},
		},
		OS: &OS{
			BootLoader: BootLoader{
				ResetType: "hard-reset",
			},
		},
	}
	err := config.IsValid()
	assert.NoError(t, err)
}

func TestConfigIsValidVerityPartitionNotFound(t *testing.T) {
	config := &Config{
		Storage: Storage{
			Disks: []Disk{{
				PartitionTableType: "gpt",
				Partitions: []Partition{
					{
						Id: "esp",
						Size: PartitionSize{
							Type: PartitionSizeTypeExplicit,
							Size: 8 * diskutils.MiB,
						},
						Type: PartitionTypeESP,
					},
					{
						Id: "root",
						Size: PartitionSize{
							Type: PartitionSizeTypeExplicit,
							Size: 1 * diskutils.GiB,
						},
					},
					{
						Id: "verityhash",
						Size: PartitionSize{
							Type: PartitionSizeTypeExplicit,
							Size: 100 * diskutils.MiB,
						},
					},
				},
			}},
			BootType: "efi",
			FileSystems: []FileSystem{
				{
					DeviceId: "esp",
					Type:     "fat32",
					MountPoint: &MountPoint{
						Path: "/boot/efi",
					},
				},
				{
					DeviceId: "root",
					Type:     "ext4",
					MountPoint: &MountPoint{
						Path: "/",
					},
				},
			},
			Verity: []Verity{
				{
					Id:           "rootverity",
					Name:         "root",
					DataDeviceId: "wrongname",
					HashDeviceId: "verityhash",
				},
			},
		},
		OS: &OS{
			BootLoader: BootLoader{
				ResetType: "hard-reset",
			},
		},
	}
	err := config.IsValid()
	assert.ErrorContains(t, err, "invalid verity item at index 0:")
	assert.ErrorContains(t, err, "invalid 'dataDeviceId'")
	assert.ErrorContains(t, err, "device (wrongname) not found")
}

func TestConfigIsValidVerityNoStorage(t *testing.T) {
	config := &Config{
		Storage: Storage{
			Verity: []Verity{
				{
					Id:           "rootverity",
					Name:         "root",
					DataDeviceId: "root",
					HashDeviceId: "verityhash",
				},
			},
		},
	}
	err := config.IsValid()
	assert.ErrorContains(t, err, "cannot specify 'verity' with dataDeviceId/hashDeviceId without specifying 'disks'")
}

func TestConfigIsValid_InvalidOutputIsInvalid(t *testing.T) {
	config := &Config{
		Output: Output{
			Image: OutputImage{
				Format: ImageFormatType("xxx"),
			},
		},
	}
	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid 'output' field")
}

func TestConfigIsValidWithPreviewFeaturesAndOutputArtifacts(t *testing.T) {
	config := &Config{
		Output: Output{
			Artifacts: &Artifacts{
				Items: []OutputArtifactsItemType{
					OutputArtifactsItemUkis,
					OutputArtifactsItemShim,
					OutputArtifactsItemSystemdBoot,
				},
				Path: "/valid/path",
			},
		},
		PreviewFeatures: []PreviewFeature{
			PreviewFeatureOutputArtifacts,
		},
	}

	err := config.IsValid()
	assert.NoError(t, err)
}

func TestConfigIsValidWithMissingOutputArtifactsPreviewFeature(t *testing.T) {
	config := &Config{
		Output: Output{
			Artifacts: &Artifacts{
				Items: []OutputArtifactsItemType{
					OutputArtifactsItemUkis,
					OutputArtifactsItemShim,
					OutputArtifactsItemSystemdBoot,
				},
				Path: "/valid/path",
			},
		},
		PreviewFeatures: []PreviewFeature{}, // empty
	}

	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "the 'output-artifacts' preview feature must be enabled to use 'output.artifacts'")
}

func TestConfigIsValidWithMissingOciPreviewFeature(t *testing.T) {
	config := &Config{
		Input: Input{
			Image: InputImage{
				Oci: &OciImage{
					Uri: "mcr.microsoft.com/azurelinux/3.0/image/minimal-os:latest",
				},
			},
		},
		PreviewFeatures: []PreviewFeature{},
	}

	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "the 'input-image-oci' preview feature must be enabled to use 'input.image.oci'")
}

func TestConfigIsValidWithPreviewFeaturesAndOutputSelinuxPolicy(t *testing.T) {
	config := &Config{
		Output: Output{
			SelinuxPolicyPath: "./selinux-policy",
		},
		PreviewFeatures: []PreviewFeature{
			PreviewFeatureOutputSelinuxPolicy,
		},
	}

	err := config.IsValid()
	assert.NoError(t, err)
}

func TestConfigIsValidWithMissingOutputSelinuxPolicyPreviewFeature(t *testing.T) {
	config := &Config{
		Output: Output{
			SelinuxPolicyPath: "./selinux-policy",
		},
		PreviewFeatures: []PreviewFeature{},
	}

	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err,
		"the 'output-selinux-policy' preview feature must be enabled to use 'output.selinuxPolicyPath'")
}

func TestConfigIsValid_InvalidPreviewFeature_Fail(t *testing.T) {
	config := &Config{
		PreviewFeatures: []PreviewFeature{"invalid-feature"},
	}

	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid 'previewFeatures' item at index 0")
	assert.ErrorContains(t, err, "invalid preview feature: invalid-feature")
}

func TestConfigIsValidWithCosiCompressionLevel(t *testing.T) {
	level := 15
	config := &Config{
		Output: Output{
			Image: OutputImage{
				Cosi: CosiConfig{
					Compression: CosiCompression{
						Level: &level,
					},
				},
			},
		},
	}

	err := config.IsValid()
	assert.NoError(t, err)
}

func TestConfigIsValidWithBtrfsPreviewFeatureAndFilesystem(t *testing.T) {
	config := &Config{
		Storage: Storage{
			Disks: []Disk{{
				PartitionTableType: "gpt",
				MaxSize:            ptrutils.PtrTo(DiskSize(3 * diskutils.GiB)),
				Partitions: []Partition{
					{
						Id:    "esp",
						Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
						End:   ptrutils.PtrTo(DiskSize(9 * diskutils.MiB)),
						Type:  PartitionTypeESP,
					},
					{
						Id:    "rootfs",
						Start: ptrutils.PtrTo(DiskSize(9 * diskutils.MiB)),
					},
				},
			}},
			BootType: "efi",
			FileSystems: []FileSystem{
				{
					DeviceId: "esp",
					Type:     FileSystemTypeFat32,
					MountPoint: &MountPoint{
						Path: "/boot/efi",
					},
				},
				{
					DeviceId: "rootfs",
					Type:     FileSystemTypeBtrfs,
					MountPoint: &MountPoint{
						Path: "/",
					},
				},
			},
		},
		OS: &OS{
			BootLoader: BootLoader{
				ResetType: ResetBootLoaderTypeHard,
			},
		},
		PreviewFeatures: []PreviewFeature{PreviewFeatureBtrfs},
	}

	err := config.IsValid()
	assert.NoError(t, err)
}

func TestConfigIsValidWithBtrfsPreviewFeatureNoFilesystem(t *testing.T) {
	config := &Config{
		Storage: Storage{
			Disks: []Disk{{
				PartitionTableType: "gpt",
				MaxSize:            ptrutils.PtrTo(DiskSize(3 * diskutils.GiB)),
				Partitions: []Partition{
					{
						Id:    "esp",
						Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
						End:   ptrutils.PtrTo(DiskSize(9 * diskutils.MiB)),
						Type:  PartitionTypeESP,
					},
					{
						Id:    "rootfs",
						Start: ptrutils.PtrTo(DiskSize(9 * diskutils.MiB)),
					},
				},
			}},
			BootType: "efi",
			FileSystems: []FileSystem{
				{
					DeviceId: "esp",
					Type:     FileSystemTypeFat32,
					MountPoint: &MountPoint{
						Path: "/boot/efi",
					},
				},
				{
					DeviceId: "rootfs",
					Type:     FileSystemTypeExt4,
					MountPoint: &MountPoint{
						Path: "/",
					},
				},
			},
		},
		OS: &OS{
			BootLoader: BootLoader{
				ResetType: ResetBootLoaderTypeHard,
			},
		},
		PreviewFeatures: []PreviewFeature{PreviewFeatureBtrfs},
	}

	err := config.IsValid()
	assert.NoError(t, err)
}

func TestConfigIsValidWithBtrfsFilesystemNoPreviewFeature(t *testing.T) {
	config := &Config{
		Storage: Storage{
			Disks: []Disk{{
				PartitionTableType: "gpt",
				MaxSize:            ptrutils.PtrTo(DiskSize(3 * diskutils.GiB)),
				Partitions: []Partition{
					{
						Id:    "esp",
						Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
						End:   ptrutils.PtrTo(DiskSize(9 * diskutils.MiB)),
						Type:  PartitionTypeESP,
					},
					{
						Id:    "rootfs",
						Start: ptrutils.PtrTo(DiskSize(9 * diskutils.MiB)),
					},
				},
			}},
			BootType: "efi",
			FileSystems: []FileSystem{
				{
					DeviceId: "esp",
					Type:     FileSystemTypeFat32,
					MountPoint: &MountPoint{
						Path: "/boot/efi",
					},
				},
				{
					DeviceId: "rootfs",
					Type:     FileSystemTypeBtrfs,
					MountPoint: &MountPoint{
						Path: "/",
					},
				},
			},
		},
		OS: &OS{
			BootLoader: BootLoader{
				ResetType: ResetBootLoaderTypeHard,
			},
		},
		PreviewFeatures: []PreviewFeature{},
	}

	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "the 'btrfs' preview feature must be enabled to use btrfs filesystems")
}
