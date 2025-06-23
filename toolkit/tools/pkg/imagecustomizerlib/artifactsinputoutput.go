// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

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

func outputArtifacts(items []imagecustomizerapi.OutputArtifactsItemType,
	outputDir string, buildDir string, buildImage string, verityMetadata []verityDeviceMetadata,
) error {
	logger.Log.Infof("Outputting artifacts")

	var outputArtifactsMetadata []imagecustomizerapi.InjectArtifactMetadata

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

	bootPartition, err := findBootPartitionFromEsp(systemBootPartition, diskPartitions, buildDir)
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

	espInjectFilePartition := imagecustomizerapi.InjectFilePartition{
		MountIdType: imagecustomizerapi.MountIdentifierTypePartUuid,
		Id:          systemBootPartition.PartUuid,
	}

	bootInjectFilePartition := imagecustomizerapi.InjectFilePartition{
		MountIdType: imagecustomizerapi.MountIdentifierTypePartUuid,
		Id:          bootPartition.PartUuid,
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

				signedName := replaceSuffix(entry.Name(), ".efi", ".signed.efi")
				source := "./" + signedName
				unsignedSource := "./" + entry.Name()

				outputArtifactsMetadata = append(outputArtifactsMetadata, imagecustomizerapi.InjectArtifactMetadata{
					Partition:      espInjectFilePartition,
					Source:         source,
					Destination:    filepath.Join("/", UkiOutputDir, entry.Name()),
					UnsignedSource: unsignedSource,
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
			return fmt.Errorf("failed to copy binary from (%s) to (%s):\n%w", srcPath, destPath, err)
		}

		signedPath := "./" + replaceSuffix(bootConfig.bootBinary, ".efi", ".signed.efi")

		outputArtifactsMetadata = append(outputArtifactsMetadata, imagecustomizerapi.InjectArtifactMetadata{
			Partition:      espInjectFilePartition,
			Source:         signedPath,
			Destination:    filepath.Join("/", ShimDir, bootConfig.bootBinary),
			UnsignedSource: "./" + bootConfig.bootBinary,
		})
	}

	// Output systemd-boot
	if slices.Contains(items, imagecustomizerapi.OutputArtifactsItemSystemdBoot) {
		srcPath := filepath.Join(systemBootPartitionTmpDir, SystemdBootDir, bootConfig.systemdBootBinary)
		destPath := filepath.Join(outputDir, bootConfig.systemdBootBinary)
		err := file.Copy(srcPath, destPath)
		if err != nil {
			return fmt.Errorf("failed to copy binary from (%s) to (%s):\n%w", srcPath, destPath, err)
		}

		signedPath := "./" + replaceSuffix(bootConfig.systemdBootBinary, ".efi", ".signed.efi")

		outputArtifactsMetadata = append(outputArtifactsMetadata, imagecustomizerapi.InjectArtifactMetadata{
			Partition:      espInjectFilePartition,
			Source:         signedPath,
			Destination:    filepath.Join("/", SystemdBootDir, bootConfig.systemdBootBinary),
			UnsignedSource: "./" + bootConfig.systemdBootBinary,
		})
	}

	// Output verity hash
	if slices.Contains(items, imagecustomizerapi.OutputArtifactsItemVerityHash) {
		for _, verity := range verityMetadata {
			if verity.hashSignaturePath == "" {
				continue
			}

			unsignedHashFile := verity.name + ".hash"
			destPath := filepath.Join(outputDir, unsignedHashFile)
			err = file.Write(verity.rootHash, destPath)
			if err != nil {
				return fmt.Errorf("failed to dump root hash for (%s) to (%s):\n%w", verity.name, destPath, err)
			}

			signedHashFile := replaceSuffix(unsignedHashFile, ".hash", ".hash.sig")
			destination := strings.TrimPrefix(verity.hashSignaturePath, "/boot")
			outputArtifactsMetadata = append(outputArtifactsMetadata, imagecustomizerapi.InjectArtifactMetadata{
				Partition:      bootInjectFilePartition,
				Source:         "./" + signedHashFile,
				Destination:    filepath.Join("/", destination),
				UnsignedSource: "./" + unsignedHashFile,
			})
		}
	}

	err = writeInjectFilesYaml(outputArtifactsMetadata, outputDir)
	if err != nil {
		return fmt.Errorf("failed to write inject-files.yaml:\n%w", err)
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

func replaceSuffix(input string, oldSuffix string, newSuffix string) string {
	if !strings.HasSuffix(input, oldSuffix) {
		return input
	}
	return strings.TrimSuffix(input, oldSuffix) + newSuffix
}

func writeInjectFilesYaml(metadata []imagecustomizerapi.InjectArtifactMetadata, outputDir string) error {
	yamlStruct := imagecustomizerapi.InjectFilesConfig{
		InjectFiles:     metadata,
		PreviewFeatures: []imagecustomizerapi.PreviewFeature{imagecustomizerapi.PreviewFeatureInjectFiles},
	}

	yamlBytes, err := yaml.Marshal(&yamlStruct)
	if err != nil {
		return fmt.Errorf("failed to marshal inject files metadata: %w", err)
	}

	outputFilePath := filepath.Join(outputDir, "inject-files.yaml")
	if err := os.WriteFile(outputFilePath, yamlBytes, 0o644); err != nil {
		return fmt.Errorf("failed to write inject-files.yaml: %w", err)
	}

	return nil
}

func InjectFilesWithConfigFile(buildDir string, configFile string, inputImageFile string,
	outputImageFile string, outputImageFormat string,
) error {
	var injectConfig imagecustomizerapi.InjectFilesConfig
	err := imagecustomizerapi.UnmarshalYamlFile(configFile, &injectConfig)
	if err != nil {
		return err
	}

	if err := injectConfig.IsValid(); err != nil {
		return fmt.Errorf("invalid inject files config: %w", err)
	}

	baseConfigPath, _ := filepath.Split(configFile)

	absBaseConfigPath, err := filepath.Abs(baseConfigPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path of inject-files.yaml:\n%w", err)
	}

	err = InjectFiles(buildDir, absBaseConfigPath, inputImageFile, injectConfig.InjectFiles,
		outputImageFile, outputImageFormat)
	if err != nil {
		return err
	}

	return nil
}

func InjectFiles(buildDir string, baseConfigPath string, inputImageFile string,
	metadata []imagecustomizerapi.InjectArtifactMetadata, outputImageFile string,
	outputImageFormat string,
) error {
	logger.Log.Debugf("Injecting Files")

	buildDirAbs, err := filepath.Abs(buildDir)
	if err != nil {
		return err
	}
	inputImageFormat := strings.TrimLeft(filepath.Ext(inputImageFile), ".")
	rawImageFile := filepath.Join(buildDirAbs, BaseImageName)

	detectedImageFormat, err := convertImageToRaw(inputImageFile, inputImageFormat, rawImageFile)
	if err != nil {
		return err
	}
	if outputImageFormat != "" {
		detectedImageFormat = imagecustomizerapi.ImageFormatType(outputImageFormat)
	}

	if outputImageFile == "" {
		outputImageFile = inputImageFile
	}

	loopback, err := safeloopback.NewLoopback(rawImageFile)
	if err != nil {
		return fmt.Errorf("failed to connect to image file to inject files:\n%w", err)
	}
	defer loopback.Close()

	diskPartitions, err := diskutils.GetDiskPartitions(loopback.DevicePath())
	if err != nil {
		return err
	}

	partitionsToMountpoints := make(map[imagecustomizerapi.InjectFilePartition]string)
	var mountedPartitions []*safemount.Mount

	for idx, item := range metadata {
		partitionKey := item.Partition
		if _, exists := partitionsToMountpoints[partitionKey]; !exists {
			partitionsToMountpoints[partitionKey] = filepath.Join(buildDir, fmt.Sprintf("inject-partition-%d", idx))

			partition, _, err := findPartition(item.Partition.MountIdType, item.Partition.Id, diskPartitions, buildDir)
			if err != nil {
				return err
			}

			mount, err := safemount.NewMount(partition.Path, partitionsToMountpoints[partitionKey], partition.FileSystemType, 0, "", true)
			if err != nil {
				return fmt.Errorf("failed to mount partition (%s):\n%w", partition.Path, err)
			}
			defer mount.Close()

			mountedPartitions = append(mountedPartitions, mount)
		}

		srcPath := filepath.Join(baseConfigPath, item.Source)
		destPath := filepath.Join(partitionsToMountpoints[partitionKey], item.Destination)
		err := file.Copy(srcPath, destPath)
		if err != nil {
			return fmt.Errorf("failed to copy binary from (%s) to (%s):\n%w", srcPath, destPath, err)
		}
	}

	for _, m := range mountedPartitions {
		if err := m.CleanClose(); err != nil {
			return fmt.Errorf("failed to cleanly unmount (%s):\n%w", m.Target(), err)
		}
	}

	err = loopback.CleanClose()
	if err != nil {
		return err
	}

	if detectedImageFormat == imagecustomizerapi.ImageFormatTypeCosi {
		partUuidToFstabEntry, baseImageVerityMetadata, osRelease, osPackages, imageUuid, imageUuidStr, cosiBootMetadata, err :=
			prepareImageConversionData(rawImageFile, buildDir, "imageroot")
		if err != nil {
			return err
		}

		err = convertToCosi(buildDirAbs, rawImageFile, outputImageFile, partUuidToFstabEntry,
			baseImageVerityMetadata, osRelease, osPackages, imageUuid, imageUuidStr, cosiBootMetadata)
		if err != nil {
			return fmt.Errorf("failed to convert customized raw image to cosi output format:\n%w", err)
		}
	} else {
		err = convertImageFile(rawImageFile, outputImageFile, detectedImageFormat)
		if err != nil {
			return fmt.Errorf("failed to convert customized raw image to output format:\n%w", err)
		}
	}

	logger.Log.Infof("Success!")

	return nil
}

func prepareImageConversionData(rawImageFile string, buildDir string,
	chrootDir string,
) (map[string]diskutils.FstabEntry, []verityDeviceMetadata, string,
	[]OsPackage, [UuidSize]byte, string, *CosiBootloader, error,
) {
	imageConnection, partUuidToFstabEntry, baseImageVerityMetadata, err := connectToExistingImage(
		rawImageFile, buildDir, chrootDir, true, true)
	if err != nil {
		return nil, nil, "", nil, [UuidSize]byte{}, "", nil, fmt.Errorf("failed to connect to image:\n%w", err)
	}
	defer imageConnection.Close()

	osRelease, err := extractOSRelease(imageConnection)
	if err != nil {
		return nil, nil, "", nil, [UuidSize]byte{}, "", nil, err
	}

	osPackages, cosiBootMetadata, err := collectOSInfo(buildDir, imageConnection)
	if err != nil {
		return nil, nil, "", nil, [UuidSize]byte{}, "", nil, err
	}

	imageUuid, imageUuidStr, err := extractImageUUID(imageConnection)
	if err != nil {
		return nil, nil, "", nil, [UuidSize]byte{}, "", nil, err
	}

	if err := imageConnection.CleanClose(); err != nil {
		return nil, nil, "", nil, [UuidSize]byte{}, "", nil, fmt.Errorf("failed to cleanly close image connection:\n%w", err)
	}

	return partUuidToFstabEntry, baseImageVerityMetadata, osRelease, osPackages, imageUuid, imageUuidStr, cosiBootMetadata, nil
}
