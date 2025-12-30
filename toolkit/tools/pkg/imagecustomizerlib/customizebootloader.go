// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/sliceutils"
	"go.opentelemetry.io/otel"
)

var (
	// Bootloader-related errors
	ErrBootloaderHardReset              = NewImageCustomizerError("Bootloader:HardReset", "failed to hard reset bootloader")
	ErrBootloaderKernelCommandLineAdd   = NewImageCustomizerError("Bootloader:KernelCommandLineAdd", "failed to add kernel command line")
	ErrBootloaderSelinuxModeGet         = NewImageCustomizerError("Bootloader:SelinuxModeGet", "failed to get existing SELinux mode")
	ErrBootloaderRootFilesystemFind     = NewImageCustomizerError("Bootloader:RootFilesystemFind", "failed to find root filesystem (i.e. mount equal to '/')")
	ErrBootloaderRootMountIdTypeGet     = NewImageCustomizerError("Bootloader:RootMountIdTypeGet", "failed to get image's root mount ID type")
	ErrBootloaderImageBootTypeGet       = NewImageCustomizerError("Bootloader:ImageBootTypeGet", "failed to get image's boot type")
	ErrBootloaderDiskConfigure          = NewImageCustomizerError("Bootloader:DiskConfigure", "failed to configure bootloader")
	ErrBootloaderRootMountFind          = NewImageCustomizerError("Bootloader:RootMountFind", "failed to find root mount (/)")
	ErrBootloaderRootMountSourceParse   = NewImageCustomizerError("Bootloader:RootMountSourceParse", "failed to parse root (/) mount source")
	ErrBootloaderUnsupportedRootMountId = NewImageCustomizerError("Bootloader:UnsupportedRootMountId", "unsupported root mount identifier")
)

func handleBootLoader(ctx context.Context, rc *ResolvedConfig, imageConnection *imageconnection.ImageConnection,
	partitionsLayout []fstabEntryPartNum, newImage bool,
) error {
	switch {
	case rc.BootLoader.ResetType == imagecustomizerapi.ResetBootLoaderTypeHard || newImage:
		err := hardResetBootLoader(ctx, rc, imageConnection, partitionsLayout, newImage)
		if err != nil {
			return fmt.Errorf("%w:\n%w", ErrBootloaderHardReset, err)
		}

	default:
		// Append the kernel command-line args to the existing grub config.
		err := AddKernelCommandLine(ctx, rc.OsKernelCommandLine.ExtraCommandLine, imageConnection.Chroot())
		if err != nil {
			return fmt.Errorf("%w:\n%w", ErrBootloaderKernelCommandLineAdd, err)
		}
	}

	return nil
}

func hardResetBootLoader(ctx context.Context, rc *ResolvedConfig, imageConnection *imageconnection.ImageConnection,
	partitionsLayout []fstabEntryPartNum, newImage bool,
) error {
	var err error
	logger.Log.Infof("Hard reset bootloader config")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "hard_reset_bootloader")
	defer span.End()

	// If the image is being customized by imagecreator, we don't need to check the SELinux mode.
	currentSelinuxMode := imagecustomizerapi.SELinuxModeDisabled

	if !newImage {
		bootCustomizer, err := NewBootCustomizer(imageConnection.Chroot())
		if err != nil {
			return err
		}

		currentSelinuxMode, err = bootCustomizer.GetSELinuxMode(rc.BuildDirAbs, imageConnection.Chroot())
		if err != nil {
			return fmt.Errorf("%w:\n%w", ErrBootloaderSelinuxModeGet, err)
		}
	}

	var rootMountIdType imagecustomizerapi.MountIdentifierType
	var bootType imagecustomizerapi.BootType
	if rc.Config.CustomizePartitions() {
		rootFileSystem, foundRootFileSystem := sliceutils.FindValueFunc(rc.Config.Storage.FileSystems,
			func(fileSystem imagecustomizerapi.FileSystem) bool {
				return fileSystem.MountPoint != nil &&
					fileSystem.MountPoint.Path == "/"
			},
		)
		if !foundRootFileSystem {
			return ErrBootloaderRootFilesystemFind
		}

		rootMountIdType = rootFileSystem.MountPoint.IdType
		bootType = rc.Config.Storage.BootType
	} else {
		rootMountIdType, err = findRootMountIdType(partitionsLayout)
		if err != nil {
			return fmt.Errorf("%w:\n%w", ErrBootloaderRootMountIdTypeGet, err)
		}

		bootType, err = getImageBootType(imageConnection)
		if err != nil {
			return fmt.Errorf("%w:\n%w", ErrBootloaderImageBootTypeGet, err)
		}
	}

	// Hard-reset the grub config.
	err = configureDiskBootLoader(imageConnection, rootMountIdType, bootType, rc.SELinux,
		rc.Config.OS.KernelCommandLine, currentSelinuxMode, newImage)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrBootloaderDiskConfigure, err)
	}

	return nil
}

// Inserts new kernel command-line args into the grub config file.
func AddKernelCommandLine(ctx context.Context, extraCommandLine []string,
	imageChroot safechroot.ChrootInterface,
) error {
	var err error

	if len(extraCommandLine) == 0 {
		// Nothing to do.
		return nil
	}

	logger.Log.Infof("Setting KernelCommandLine.ExtraCommandLine")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "add_kernel_command_line")
	defer span.End()

	bootCustomizer, err := NewBootCustomizer(imageChroot)
	if err != nil {
		return err
	}

	err = bootCustomizer.AddKernelCommandLine(extraCommandLine)
	if err != nil {
		return err
	}

	err = bootCustomizer.WriteToFile(imageChroot)
	if err != nil {
		return err
	}

	return nil
}

func findRootMountIdType(partitionsLayout []fstabEntryPartNum,
) (imagecustomizerapi.MountIdentifierType, error) {
	rootFound := false
	rootFstabEntry := diskutils.FstabEntry{}
	for _, entry := range partitionsLayout {
		fstabEntry := entry.FstabEntry
		if fstabEntry.Target == "/" {
			rootFound = true
			rootFstabEntry = fstabEntry
			break
		}
	}

	if !rootFound {
		return "", ErrBootloaderRootMountFind
	}

	mountIdType, mountId, err := parseExtendedSourcePartition(rootFstabEntry.Source)
	if err != nil {
		return "", fmt.Errorf("%w:\n%w", ErrBootloaderRootMountSourceParse, err)
	}

	rootMountIdType := imagecustomizerapi.MountIdentifierTypeDefault
	switch mountIdType {
	case ExtendedMountIdentifierTypeUuid:
		rootMountIdType = imagecustomizerapi.MountIdentifierTypeUuid

	case ExtendedMountIdentifierTypePartUuid:
		rootMountIdType = imagecustomizerapi.MountIdentifierTypePartUuid

	case ExtendedMountIdentifierTypePartLabel:
		rootMountIdType = imagecustomizerapi.MountIdentifierTypePartLabel

	case ExtendedMountIdentifierTypeDev:
		switch mountId {
		case imagecustomizerapi.VerityRootDevicePath:
			// The root partition is a verity partition.
			// The verity settings will override this value when verity is applied. So, just use the default value.
			rootMountIdType = imagecustomizerapi.MountIdentifierTypeDefault

		default:
			return "", fmt.Errorf("%w (identifier='%s')", ErrBootloaderUnsupportedRootMountId, rootFstabEntry.Source)
		}

	default:
		return "", fmt.Errorf("%w (identifier='%s')", ErrBootloaderUnsupportedRootMountId, rootFstabEntry.Source)
	}

	return rootMountIdType, nil
}
