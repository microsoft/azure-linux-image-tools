// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

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

type OutputArtifactMetadata struct {
	Partition   InjectFilePartition `yaml:"partition"`
	Destination string              `yaml:"destination"`
	Source      string              `yaml:"source"`
}

type InjectFilePartition struct {
	MountIdType imagecustomizerapi.MountIdentifierType `yaml:"mountIdType"`
	Id          string                                 `yaml:"id"`
}

type InjectFilesYaml struct {
	InjectFiles []OutputArtifactMetadata `yaml:"injectFiles"`
}

var ukiRegex = regexp.MustCompile(`^vmlinuz-.*\.efi$`)

func outputArtifacts(items []imagecustomizerapi.OutputArtifactsItemType,
	configOutputArtifactsPath string, buildDir string, buildImage string, baseConfigPath string,
) ([]OutputArtifactMetadata, error) {
	logger.Log.Infof("Outputting artifacts")

	var outputArtifactsMetadata []OutputArtifactMetadata
	outputDir := file.GetAbsPathWithBase(baseConfigPath, configOutputArtifactsPath)
	prefix := ""
	if !filepath.IsAbs(configOutputArtifactsPath) {
		prefix = "./"
	}

	loopback, err := safeloopback.NewLoopback(buildImage)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to image file to output artifacts:\n%w", err)
	}
	defer loopback.Close()

	diskPartitions, err := diskutils.GetDiskPartitions(loopback.DevicePath())
	if err != nil {
		return nil, err
	}

	systemBootPartition, err := findSystemBootPartition(diskPartitions)
	if err != nil {
		return nil, err
	}

	systemBootPartitionTmpDir := filepath.Join(buildDir, tmpEspPartitionDirName)
	systemBootPartitionMount, err := safemount.NewMount(systemBootPartition.Path,
		systemBootPartitionTmpDir, systemBootPartition.FileSystemType, unix.MS_RDONLY, "", true)
	if err != nil {
		return nil, fmt.Errorf("failed to mount esp partition (%s):\n%w", systemBootPartition.Path, err)
	}
	defer systemBootPartitionMount.Close()

	// Detect system architecture
	_, bootConfig, err := getBootArchConfig()
	if err != nil {
		return nil, err
	}

	partition := InjectFilePartition{
		MountIdType: imagecustomizerapi.MountIdentifierTypePartUuid,
		Id:          systemBootPartition.PartUuid,
	}

	// Output UKIs
	if slices.Contains(items, imagecustomizerapi.OutputArtifactsItemUkis) {
		ukiDir := filepath.Join(systemBootPartitionTmpDir, UkiOutputDir)
		dirEntries, err := os.ReadDir(ukiDir)
		if err != nil {
			return nil, fmt.Errorf("failed to read UKI directory (%s):\n%w", ukiDir, err)
		}

		for _, entry := range dirEntries {
			if !entry.IsDir() && ukiRegex.MatchString(entry.Name()) {
				srcPath := filepath.Join(ukiDir, entry.Name())
				destPath := filepath.Join(outputDir, entry.Name())
				err := file.Copy(srcPath, destPath)
				if err != nil {
					return nil, fmt.Errorf("failed to copy binary from (%s) to (%s):\n%w", srcPath, destPath, err)
				}

				signedName := replaceSuffix(entry.Name(), ".unsigned.efi", ".signed.efi")
				source := prefix + filepath.Join(configOutputArtifactsPath, signedName)
				destinationName := replaceSuffix(entry.Name(), ".unsigned.efi", ".efi")

				outputArtifactsMetadata = append(outputArtifactsMetadata, OutputArtifactMetadata{
					Partition:   partition,
					Source:      source,
					Destination: filepath.Join("/EFI/Linux", destinationName),
				})
			}
		}
	}

	// Output shim
	if slices.Contains(items, imagecustomizerapi.OutputArtifactsItemShim) {
		srcPath := filepath.Join(systemBootPartitionTmpDir, ShimDir, bootConfig.bootBinary)
		destPath := filepath.Join(outputDir, bootConfig.bootBinary)
		err := file.Copy(srcPath, destPath)
		if err != nil {
			return nil, fmt.Errorf("failed to copy binary from (%s) to (%s):\n%w", srcPath, destPath, err)
		}

		signedPath := prefix + replaceSuffix(filepath.Join(configOutputArtifactsPath, bootConfig.bootBinary), ".efi", ".signed.efi")

		outputArtifactsMetadata = append(outputArtifactsMetadata, OutputArtifactMetadata{
			Partition:   partition,
			Source:      signedPath,
			Destination: filepath.Join("/EFI/BOOT", bootConfig.bootBinary),
		})
	}

	// Output systemd-boot
	if slices.Contains(items, imagecustomizerapi.OutputArtifactsItemSystemdBoot) {
		srcPath := filepath.Join(systemBootPartitionTmpDir, SystemdBootDir, bootConfig.systemdBootBinary)
		destPath := filepath.Join(outputDir, bootConfig.systemdBootBinary)
		err := file.Copy(srcPath, destPath)
		if err != nil {
			return nil, fmt.Errorf("failed to copy binary from (%s) to (%s):\n%w", srcPath, destPath, err)
		}

		signedPath := prefix + replaceSuffix(filepath.Join(configOutputArtifactsPath, bootConfig.systemdBootBinary), ".efi", ".signed.efi")

		outputArtifactsMetadata = append(outputArtifactsMetadata, OutputArtifactMetadata{
			Partition:   partition,
			Source:      signedPath,
			Destination: filepath.Join("/EFI/systemd", bootConfig.systemdBootBinary),
		})
	}

	err = systemBootPartitionMount.CleanClose()
	if err != nil {
		return nil, err
	}

	err = loopback.CleanClose()
	if err != nil {
		return nil, err
	}

	return outputArtifactsMetadata, nil
}

func replaceSuffix(input string, oldSuffix string, newSuffix string) string {
	if !strings.HasSuffix(input, oldSuffix) {
		return input
	}
	return strings.TrimSuffix(input, oldSuffix) + newSuffix
}

func writeInjectFilesYaml(metadata []OutputArtifactMetadata, baseConfigPath string) error {
	type injectFileYaml struct {
		InjectFiles []OutputArtifactMetadata `yaml:"injectFiles"`
	}

	yamlStruct := injectFileYaml{
		InjectFiles: metadata,
	}

	// Marshal YAML
	yamlBytes, err := yaml.Marshal(&yamlStruct)
	if err != nil {
		return fmt.Errorf("failed to marshal inject files metadata: %w", err)
	}

	// Construct YAML with header and inline unsigned comments
	var builder strings.Builder
	builder.WriteString("# This file is generated automatically by Prism output artifacts API.\n\n")

	lines := strings.Split(string(yamlBytes), "\n")
	i := 0
	for _, item := range metadata {
		for ; i < len(lines); i++ {
			line := lines[i]
			builder.WriteString(line + "\n")

			// When you hit the 'source:' line, add the comment line
			if strings.TrimSpace(line) == "source: "+item.Source {
				indent := strings.Repeat(" ", leadingSpaces(line))
				unsignedSource := replaceSuffix(item.Source, ".signed.efi", ".unsigned.efi")
				builder.WriteString(fmt.Sprintf("%s# unsigned source: %s\n", indent, unsignedSource))
				i++
				break
			}
		}
	}

	outputFilePath := filepath.Join(baseConfigPath, "inject-files.yaml")
	if err := os.WriteFile(outputFilePath, []byte(builder.String()), 0644); err != nil {
		return fmt.Errorf("failed to write inject-files.yaml: %w", err)
	}

	return nil
}

func leadingSpaces(s string) int {
	return len(s) - len(strings.TrimLeft(s, " "))
}

func (i *InjectFilesYaml) IsValid() error {
	if len(i.InjectFiles) == 0 {
		return fmt.Errorf("injectFiles is empty")
	}
	for idx, entry := range i.InjectFiles {
		if entry.Source == "" || entry.Destination == "" {
			return fmt.Errorf("injectFiles[%d] has empty source or destination", idx)
		}
		if entry.Partition.Id == "" {
			return fmt.Errorf("injectFiles[%d] has empty partition id", idx)
		}
		if err := entry.Partition.MountIdType.IsValid(); err != nil {
			return fmt.Errorf("injectFiles[%d] has invalid partition mount id type: %w", idx, err)
		}
	}
	return nil
}

func InjectFiles(buildDir string, baseConfigPath string, inputImageFile string,
	metadata []OutputArtifactMetadata,
) error {
	logger.Log.Debugf("Injecting Files")

	loopback, err := safeloopback.NewLoopback(inputImageFile)
	if err != nil {
		return fmt.Errorf("failed to connect to image file to inject files:\n%w", err)
	}
	defer loopback.Close()

	diskPartitions, err := diskutils.GetDiskPartitions(loopback.DevicePath())
	if err != nil {
		return err
	}

	uniqueKeys := make(map[InjectFilePartition]bool)
	var partitions []diskutils.PartitionInfo

	for _, item := range metadata {
		key := InjectFilePartition{MountIdType: item.Partition.MountIdType, Id: item.Partition.Id}
		if uniqueKeys[key] {
			continue
		}
		uniqueKeys[key] = true

		partition, _, err := findPartition(item.Partition.MountIdType, item.Partition.Id, diskPartitions, buildDir)
		if err != nil {
			return err
		}

		partitions = append(partitions, partition)
		logger.Log.Infof("Identified partition %s (%s) -> device %s", item.Partition.Id, item.Partition.MountIdType, partition.Path)
	}

	// Next step: Mount & inject (to be added later)
	// ...

	err = loopback.CleanClose()
	if err != nil {
		return err
	}

	return nil
}
