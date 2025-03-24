// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safemount"
	"golang.org/x/sys/unix"
)

const (
	ShimDir        = "EFI/BOOT"
	SystemdBootDir = "EFI/systemd"
)

var ukiRegex = regexp.MustCompile(`^vmlinuz-.*\.efi$`)

func outputArtifacts(items []imagecustomizerapi.OutputArtifactsItemType, outputDir string, buildDir string, buildImage string) error {
	logger.Log.Infof("Outputting artifacts")

	loopback, err := safeloopback.NewLoopback(buildImage)
	if err != nil {
		return fmt.Errorf("failed to connect to image file to output artifacts:\n%w", err)
	}
	defer loopback.Close()

	diskPartitions, err := diskutils.GetDiskPartitions(loopback.DevicePath())
	if err != nil {
		return err
	}

	systemBootPartition, err := findSystemBootPartition(diskPartitions)
	if err != nil {
		return err
	}

	systemBootPartitionTmpDir := filepath.Join(buildDir, tmpEspPartitionDirName)
	systemBootPartitionMount, err := safemount.NewMount(systemBootPartition.Path,
		systemBootPartitionTmpDir, systemBootPartition.FileSystemType, unix.MS_RDONLY, "", true)
	if err != nil {
		return fmt.Errorf("failed to mount esp partition (%s):\n%w", systemBootPartition.Path, err)
	}
	defer systemBootPartitionMount.Close()

	// Detect system architecture
	_, bootConfig, err := getBootArchConfig()
	if err != nil {
		return err
	}

	// Output UKIs
	if slices.Contains(items, imagecustomizerapi.OutputArtifactsItemUkis) {
		ukiDir := filepath.Join(systemBootPartitionTmpDir, UkiOutputDir)
		dirEntries, err := os.ReadDir(ukiDir)
		if err != nil {
			return fmt.Errorf("failed to read UKI directory (%s):\n%w", ukiDir, err)
		}

		for _, entry := range dirEntries {
			if !entry.IsDir() && ukiRegex.MatchString(entry.Name()) {
				srcPath := filepath.Join(ukiDir, entry.Name())
				destPath := filepath.Join(outputDir, entry.Name())
				err := file.Copy(srcPath, destPath)
				if err != nil {
					return fmt.Errorf("failed to copy binary from (%s) to (%s):\n%w", srcPath, destPath, err)
				}
			}
		}
	}

	// Output shim
	if slices.Contains(items, imagecustomizerapi.OutputArtifactsItemShim) {
		srcPath := filepath.Join(systemBootPartitionTmpDir, ShimDir, bootConfig.bootBinary)
		destPath := filepath.Join(outputDir, bootConfig.bootBinary)
		err := file.Copy(srcPath, destPath)
		if err != nil {
			return fmt.Errorf("failed to copy binary from (%s) to (%s):\n%w", srcPath, destPath, err)
		}
	}

	// Output systemd-boot
	if slices.Contains(items, imagecustomizerapi.OutputArtifactsItemSystemdBoot) {
		srcPath := filepath.Join(systemBootPartitionTmpDir, SystemdBootDir, bootConfig.systemdBootBinary)
		destPath := filepath.Join(outputDir, bootConfig.systemdBootBinary)
		err := file.Copy(srcPath, destPath)
		if err != nil {
			return fmt.Errorf("failed to copy binary from (%s) to (%s):\n%w", srcPath, destPath, err)
		}
	}

	err = systemBootPartitionMount.CleanClose()
	if err != nil {
		return err
	}

	err = loopback.CleanClose()
	if err != nil {
		return err
	}

	return nil
}
