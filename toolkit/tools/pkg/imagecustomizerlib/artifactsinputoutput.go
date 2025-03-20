// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

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
	UkiPattern     = `vmlinuz-.*\.efi$`
)

func outputArtifacts(items []imagecustomizerapi.Item, outputDir string, buildDir string, buildImage string) error {
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
	systemBootPartitionMount, err := safemount.NewMount(systemBootPartition.Path, systemBootPartitionTmpDir, systemBootPartition.FileSystemType, unix.MS_RDONLY, "", true)
	if err != nil {
		return fmt.Errorf("failed to mount esp partition (%s):\n%w", systemBootPartition.Path, err)
	}
	defer systemBootPartitionMount.Close()

	// Detect system architecture
	arch := runtime.GOARCH
	var shimBinaryName, systemdBootBinaryName string

	switch arch {
	case "amd64", "x86_64":
		shimBinaryName = "bootx64.efi"
		systemdBootBinaryName = "systemd-bootx64.efi"
	case "arm64":
		shimBinaryName = "bootaa64.efi"
		systemdBootBinaryName = "systemd-bootaa64.efi"
	default:
		return fmt.Errorf("unsupported architecture: %s", arch)
	}

	ukiRegex, err := regexp.Compile(UkiPattern)
	if err != nil {
		return fmt.Errorf("failed to compile UKI regex: %w", err)
	}

	for _, item := range items {
		switch item {
		case imagecustomizerapi.ItemUkis:
			ukiDir := filepath.Join(systemBootPartitionTmpDir, UkiOutputDir)
			dirEntries, err := os.ReadDir(ukiDir)
			if err != nil {
				return fmt.Errorf("failed to read UKI directory (%s):\n%w", ukiDir, err)
			}

			for _, entry := range dirEntries {
				if !entry.IsDir() && ukiRegex.MatchString(entry.Name()) {
					srcPath := filepath.Join(ukiDir, entry.Name())
					destPath := filepath.Join(outputDir, entry.Name())
					err := extractAndCopyArtifact(srcPath, destPath)
					if err != nil {
						return err
					}
				}
			}
		case imagecustomizerapi.ItemShim:
			srcPath := filepath.Join(systemBootPartitionTmpDir, ShimDir, shimBinaryName)
			destPath := filepath.Join(outputDir, shimBinaryName)
			err := extractAndCopyArtifact(srcPath, destPath)
			if err != nil {
				return err
			}

		case imagecustomizerapi.ItemSystemdBoot:
			srcPath := filepath.Join(systemBootPartitionTmpDir, SystemdBootDir, systemdBootBinaryName)
			destPath := filepath.Join(outputDir, systemdBootBinaryName)
			err := extractAndCopyArtifact(srcPath, destPath)
			if err != nil {
				return err
			}
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

func extractAndCopyArtifact(srcPath string, destPath string) error {
	exists, err := file.PathExists(srcPath)
	if err != nil {
		return fmt.Errorf("failed to check file path (%s):\n%w", srcPath, err)
	} else if !exists {
		return fmt.Errorf("binary not found at expected location: %s", srcPath)
	}

	err = file.Copy(srcPath, destPath)
	if err != nil {
		return fmt.Errorf("failed to copy binary from (%s) to (%s):\n%w", srcPath, destPath, err)
	}

	return nil
}
