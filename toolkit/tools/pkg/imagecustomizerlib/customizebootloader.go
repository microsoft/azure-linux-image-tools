// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/installutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/sliceutils"
)

func handleBootLoader(baseConfigPath string, config *imagecustomizerapi.Config, imageConnection *ImageConnection,
	partUuidToFstabEntry map[string]diskutils.FstabEntry,
) error {

	switch config.OS.BootLoader.ResetType {
	case imagecustomizerapi.ResetBootLoaderTypeHard:
		err := hardResetBootLoader(baseConfigPath, config, imageConnection, partUuidToFstabEntry)
		if err != nil {
			return fmt.Errorf("failed to hard reset bootloader:\n%w", err)
		}

	default:
		// Append the kernel command-line args to the existing grub config.
		err := AddKernelCommandLine(config.OS.KernelCommandLine.ExtraCommandLine, imageConnection.Chroot())
		if err != nil {
			return fmt.Errorf("failed to add extra kernel command line:\n%w", err)
		}
	}

	return nil
}

func hardResetBootLoader(baseConfigPath string, config *imagecustomizerapi.Config, imageConnection *ImageConnection,
	partUuidToFstabEntry map[string]diskutils.FstabEntry,
) error {
	var err error
	logger.Log.Infof("Hard reset bootloader config")

	bootCustomizer, err := NewBootCustomizer(imageConnection.Chroot())
	if err != nil {
		return err
	}

	currentSelinuxMode, err := bootCustomizer.GetSELinuxMode(imageConnection.Chroot())
	if err != nil {
		return fmt.Errorf("failed to get existing SELinux mode:\n%w", err)
	}

	var rootMountIdType imagecustomizerapi.MountIdentifierType
	var bootType imagecustomizerapi.BootType
	if config.CustomizePartitions() {
		rootFileSystem, foundRootFileSystem := sliceutils.FindValueFunc(config.Storage.FileSystems,
			func(fileSystem imagecustomizerapi.FileSystem) bool {
				return fileSystem.MountPoint != nil &&
					fileSystem.MountPoint.Path == "/"
			},
		)
		if !foundRootFileSystem {
			return fmt.Errorf("failed to find root filesystem (i.e. mount equal to '/')")
		}

		rootMountIdType = rootFileSystem.MountPoint.IdType
		bootType = config.Storage.BootType
	} else {
		rootMountIdType, err = findRootMountIdType(partUuidToFstabEntry)
		if err != nil {
			return fmt.Errorf("failed to get image's root mount ID type:\n%w", err)
		}

		bootType, err = getImageBootType(imageConnection)
		if err != nil {
			return fmt.Errorf("failed to get image's boot type:\n%w", err)
		}
	}

	// Hard-reset the grub config.
	err = configureDiskBootLoader(imageConnection, rootMountIdType, bootType, config.OS.SELinux,
		config.OS.KernelCommandLine, currentSelinuxMode)
	if err != nil {
		return fmt.Errorf("failed to configure bootloader:\n%w", err)
	}

	return nil
}

// Inserts new kernel command-line args into the grub config file.
func AddKernelCommandLine(extraCommandLine []string,
	imageChroot safechroot.ChrootInterface,
) error {
	var err error

	if len(extraCommandLine) == 0 {
		// Nothing to do.
		return nil
	}

	logger.Log.Infof("Setting KernelCommandLine.ExtraCommandLine")

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

func findRootMountIdType(partUuidToFstabEntry map[string]diskutils.FstabEntry,
) (imagecustomizerapi.MountIdentifierType, error) {
	rootFound := false
	rootFstabEntry := diskutils.FstabEntry{}
	for _, fstabEntry := range partUuidToFstabEntry {
		if fstabEntry.Target == "/" {
			rootFound = true
			rootFstabEntry = fstabEntry
			break
		}
	}

	if !rootFound {
		return "", fmt.Errorf("failed to find root mount (/)")
	}

	mountIdType, mountId, err := parseExtendedSourcePartition(rootFstabEntry.Source)
	if err != nil {
		return "", fmt.Errorf("failed to parse root (/) mount source:\n%w", err)
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
			return "", fmt.Errorf("unsupported root mount identifier (%s)", rootFstabEntry.Source)
		}

	default:
		return "", fmt.Errorf("unsupported root mount identifier (%s)", rootFstabEntry.Source)
	}

	return rootMountIdType, nil
}

func extractCosiBootMetadata(imageConnection *ImageConnection) (*CosiBootloader, error) {
	chrootDir := imageConnection.Chroot().RootDir()

	if GrubConfigSupportExists(imageConnection.Chroot()) {
		return &CosiBootloader{
			Type:        BootloaderGrub,
			SystemdBoot: nil,
		}, nil
	}

	loaderEntryDir := filepath.Join(chrootDir, "boot", "loader", "entries")
	if exists, err := file.DirExists(loaderEntryDir); err != nil {
		return nil, fmt.Errorf("failed while checking if systemd-boot entry dir '%s' exists:\n%w", loaderEntryDir, err)
	} else if exists {
		entries, err := extractSystemdBootEntries(loaderEntryDir)
		if err != nil {
			return nil, fmt.Errorf("failed to extract systemd-boot entries:\n%w", err)
		}
		return &CosiBootloader{
			Type:        BootloaderSystemdBoot,
			SystemdBoot: &SystemDBoot{Entries: entries},
		}, nil
	}

	efiDir := filepath.Join(chrootDir, "boot", "efi", "EFI", "Linux")
	efiFiles, err := filepath.Glob(filepath.Join(efiDir, "*.efi"))
	if err != nil {
		return nil, fmt.Errorf("failed to scan UKI EFI files:\n%w", err)
	}
	if len(efiFiles) > 0 {
		var entries []SystemDBootEntry
		for _, efiPath := range efiFiles {
			kernelVer, err := getKernelNameFromUki(efiPath)
			if err != nil {
				continue
			}
			entries = append(entries, SystemDBootEntry{
				Type:    EntryTypeUKIStandalone,
				Path:    efiPath,
				Kernel:  kernelVer,
				Cmdline: "", // Not extracted for UKI standalone EFI
			})
		}
		return &CosiBootloader{
			Type:        BootloaderSystemdBoot,
			SystemdBoot: &SystemDBoot{Entries: entries},
		}, nil
	}

	return nil, nil // No known boot metadata found
}

func extractSystemdBootEntries(entryDir string) ([]SystemDBootEntry, error) {
	files, err := os.ReadDir(entryDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", entryDir, err)
	}

	var entries []SystemDBootEntry

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".conf") {
			continue
		}

		path := filepath.Join(entryDir, file.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", path, err)
		}

		entry := SystemDBootEntry{Path: path}

		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "options"):
				entry.Cmdline = strings.TrimSpace(strings.TrimPrefix(line, "options"))
			case strings.HasPrefix(line, "linux"):
				entry.Kernel = strings.TrimSpace(strings.TrimPrefix(line, "linux"))
			case strings.HasPrefix(line, "uki"):
				entry.Kernel = strings.TrimSpace(strings.TrimPrefix(line, "uki"))
				entry.Type = EntryTypeUKIConfig
			}
		}

		// Fallback for type if not set explicitly by "uki"
		if entry.Type == "" {
			if entry.Kernel != "" && strings.HasSuffix(entry.Kernel, ".efi") {
				entry.Type = EntryTypeUKIConfig
			} else if entry.Kernel != "" {
				entry.Type = EntryTypeConfig
			} else {
				entry.Type = EntryTypeConfig // default fallback
			}
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

func GrubConfigSupportExists(installChroot safechroot.ChrootInterface) bool {
	cfgPath := filepath.Join(installChroot.RootDir(), installutils.GrubCfgFile)
	defPath := filepath.Join(installChroot.RootDir(), installutils.GrubDefFile)

	cfgExists := false
	if _, err := os.Stat(cfgPath); err == nil {
		cfgExists = true
	}

	defExists := false
	if _, err := os.Stat(defPath); err == nil {
		defExists = true
	}

	return cfgExists && defExists
}
