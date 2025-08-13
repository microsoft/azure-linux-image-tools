// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"go.opentelemetry.io/otel"
	"gopkg.in/yaml.v3"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/randomization"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safemount"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/sliceutils"
	"golang.org/x/sys/unix"
)

var (
	// Artifact handling errors
	ErrArtifactImageConnection              = NewImageCustomizerError("Artifacts:ImageConnection", "failed to connect to image file to output artifacts")
	ErrArtifactESPPartitionMount            = NewImageCustomizerError("Artifacts:ESPPartitionMount", "failed to mount ESP partition")
	ErrArtifactUKIDirectoryRead             = NewImageCustomizerError("Artifacts:UKIDirectoryRead", "failed to read UKI directory")
	ErrArtifactBinaryCopy                   = NewImageCustomizerError("Artifacts:BinaryCopy", "failed to copy binary")
	ErrArtifactRootHashDump                 = NewImageCustomizerError("Artifacts:RootHashDump", "failed to dump root hash")
	ErrArtifactInjectFilesYamlWrite         = NewImageCustomizerError("Artifacts:InjectFilesYamlWrite", "failed to write inject-files.yaml")
	ErrArtifactInjectFilesYamlMarshal       = NewImageCustomizerError("Artifacts:InjectFilesYamlMarshal", "failed to marshal inject files metadata")
	ErrArtifactInvalidInjectFilesConfig     = NewImageCustomizerError("Artifacts:InvalidInjectFilesConfig", "invalid inject files config")
	ErrArtifactInjectFilesPathResolution    = NewImageCustomizerError("Artifacts:InjectFilesPathResolution", "failed to get absolute path of inject-files.yaml")
	ErrArtifactInjectFilesImageConnection   = NewImageCustomizerError("Artifacts:InjectFilesImageConnection", "failed to connect to image file to inject files")
	ErrArtifactInjectFilesPartitionMount    = NewImageCustomizerError("Artifacts:InjectFilesPartitionMount", "failed to mount partition for file injection")
	ErrArtifactPartitionUnmount             = NewImageCustomizerError("Artifacts:PartitionUnmount", "failed to cleanly unmount partition")
	ErrArtifactCosiImageConversion          = NewImageCustomizerError("Artifacts:CosiImageConversion", "failed to convert customized raw image to cosi output format")
	ErrArtifactOutputImageConversion        = NewImageCustomizerError("Artifacts:OutputImageConversion", "failed to convert customized raw image to output format")
	ErrArtifactImageConnectionForExtraction = NewImageCustomizerError("Artifacts:ImageConnectionForExtraction", "failed to connect to image")
	ErrArtifactImageConnectionClose         = NewImageCustomizerError("Artifacts:ImageConnectionClose", "failed to cleanly close image connection")
	ErrArtifactReleaseFileRead              = NewImageCustomizerError("Artifacts:ReleaseFileRead", "failed to read release file")
	ErrArtifactImageUuidNotFound            = NewImageCustomizerError("Artifacts:ImageUuidNotFound", "IMAGE_UUID not found in release file")
	ErrArtifactImageUuidParse               = NewImageCustomizerError("Artifacts:ImageUuidParse", "failed to parse IMAGE_UUID")
)

const (
	ShimDir        = "EFI/BOOT"
	SystemdBootDir = "EFI/systemd"
)

var ukiRegex = regexp.MustCompile(`^vmlinuz-.*\.efi$`)

func outputArtifacts(ctx context.Context, items []imagecustomizerapi.OutputArtifactsItemType,
	outputDir string, buildDir string, buildImage string, verityMetadata []verityDeviceMetadata,
) error {
	logger.Log.Infof("Outputting artifacts")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "output_artifacts")
	defer span.End()

	var outputArtifactsMetadata []imagecustomizerapi.InjectArtifactMetadata

	loopback, err := safeloopback.NewLoopback(buildImage)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrArtifactImageConnection, err)
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
		return fmt.Errorf("%w (partition='%s'):\n%w", ErrArtifactESPPartitionMount, systemBootPartition.Path, err)
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
			return fmt.Errorf("%w (directory='%s'):\n%w", ErrArtifactUKIDirectoryRead, ukiDir, err)
		}

		for _, entry := range dirEntries {
			if !entry.IsDir() && ukiRegex.MatchString(entry.Name()) {
				srcPath := filepath.Join(ukiDir, entry.Name())
				destPath := filepath.Join(outputDir, entry.Name())
				err := file.Copy(srcPath, destPath)
				if err != nil {
					return fmt.Errorf("%w (source='%s', destination='%s'):\n%w", ErrArtifactBinaryCopy, srcPath, destPath, err)
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
			return fmt.Errorf("%w (source='%s', destination='%s'):\n%w", ErrArtifactBinaryCopy, srcPath, destPath, err)
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
			return fmt.Errorf("%w (source='%s', destination='%s'):\n%w", ErrArtifactBinaryCopy, srcPath, destPath, err)
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
				return fmt.Errorf("%w (name='%s', path='%s'):\n%w", ErrArtifactRootHashDump, verity.name, destPath, err)
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
		return fmt.Errorf("%w (outputDir='%s'):\n%w", ErrArtifactInjectFilesYamlWrite, outputDir, err)
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
		return fmt.Errorf("%w:\n%w", ErrArtifactInjectFilesYamlMarshal, err)
	}

	outputFilePath := filepath.Join(outputDir, "inject-files.yaml")
	if err := os.WriteFile(outputFilePath, yamlBytes, 0o644); err != nil {
		return fmt.Errorf("%w (file='%s'):\n%w", ErrArtifactInjectFilesYamlWrite, outputFilePath, err)
	}

	return nil
}

func InjectFilesWithConfigFile(ctx context.Context, buildDir string, configFile string, inputImageFile string,
	outputImageFile string, outputImageFormat string,
) error {
	var injectConfig imagecustomizerapi.InjectFilesConfig
	err := imagecustomizerapi.UnmarshalYamlFile(configFile, &injectConfig)
	if err != nil {
		return err
	}

	if err := injectConfig.IsValid(); err != nil {
		return fmt.Errorf("%w:\n%w", ErrArtifactInvalidInjectFilesConfig, err)
	}

	baseConfigPath, _ := filepath.Split(configFile)

	absBaseConfigPath, err := filepath.Abs(baseConfigPath)
	if err != nil {
		return fmt.Errorf("%w (path='%s'):\n%w", ErrArtifactInjectFilesPathResolution, baseConfigPath, err)
	}

	err = InjectFiles(ctx, buildDir, absBaseConfigPath, inputImageFile, injectConfig.InjectFiles,
		outputImageFile, outputImageFormat)
	if err != nil {
		return err
	}

	return nil
}

func InjectFiles(ctx context.Context, buildDir string, baseConfigPath string, inputImageFile string,
	metadata []imagecustomizerapi.InjectArtifactMetadata, outputImageFile string,
	outputImageFormat string,
) error {
	logger.Log.Debugf("Injecting Files")

	ctx, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "inject_files")
	defer span.End()

	buildDirAbs, err := filepath.Abs(buildDir)
	if err != nil {
		return err
	}
	rawImageFile := filepath.Join(buildDirAbs, BaseImageName)

	detectedImageFormat, err := convertImageToRaw(inputImageFile, rawImageFile)
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
		return fmt.Errorf("%w:\n%w", ErrArtifactInjectFilesImageConnection, err)
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
				return fmt.Errorf("%w (partition='%s'):\n%w", ErrArtifactInjectFilesPartitionMount, partition.Path, err)
			}
			defer mount.Close()

			mountedPartitions = append(mountedPartitions, mount)
		}

		srcPath := filepath.Join(baseConfigPath, item.Source)
		destPath := filepath.Join(partitionsToMountpoints[partitionKey], item.Destination)
		err := file.Copy(srcPath, destPath)
		if err != nil {
			return fmt.Errorf("%w (source='%s', destination='%s'):\n%w", ErrArtifactBinaryCopy, srcPath, destPath, err)
		}
	}

	for _, m := range mountedPartitions {
		if err := m.CleanClose(); err != nil {
			return fmt.Errorf("%w (target='%s'):\n%w", ErrArtifactPartitionUnmount, m.Target(), err)
		}
	}

	err = loopback.CleanClose()
	if err != nil {
		return err
	}

	if detectedImageFormat == imagecustomizerapi.ImageFormatTypeCosi {
		partUuidToFstabEntry, baseImageVerityMetadata, osRelease, osPackages, imageUuid, imageUuidStr, cosiBootMetadata, err := prepareImageConversionData(ctx, rawImageFile, buildDir, "imageroot")
		if err != nil {
			return err
		}

		err = convertToCosi(buildDirAbs, rawImageFile, outputImageFile, partUuidToFstabEntry,
			baseImageVerityMetadata, osRelease, osPackages, imageUuid, imageUuidStr, cosiBootMetadata)
		if err != nil {
			return fmt.Errorf("%w (output='%s'):\n%w", ErrArtifactCosiImageConversion, outputImageFile, err)
		}
	} else {
		err = ConvertImageFile(rawImageFile, outputImageFile, detectedImageFormat)
		if err != nil {
			return fmt.Errorf("%w (output='%s', format='%s'):\n%w", ErrArtifactOutputImageConversion, outputImageFile, detectedImageFormat, err)
		}
	}

	logger.Log.Infof("Success!")

	return nil
}

func prepareImageConversionData(ctx context.Context, rawImageFile string, buildDir string,
	chrootDir string,
) (map[string]diskutils.FstabEntry, []verityDeviceMetadata, string,
	[]OsPackage, [randomization.UuidSize]byte, string, *CosiBootloader, error,
) {
	imageConnection, partUuidToFstabEntry, baseImageVerityMetadata, _, err := connectToExistingImage(ctx,
		rawImageFile, buildDir, chrootDir, true, true, false)
	if err != nil {
		return nil, nil, "", nil, [randomization.UuidSize]byte{}, "", nil, fmt.Errorf("%w:\n%w", ErrArtifactImageConnectionForExtraction, err)
	}
	defer imageConnection.Close()

	osRelease, err := extractOSRelease(imageConnection)
	if err != nil {
		return nil, nil, "", nil, [randomization.UuidSize]byte{}, "", nil, err
	}

	osPackages, cosiBootMetadata, err := collectOSInfoHelper(ctx, buildDir, imageConnection)
	if err != nil {
		return nil, nil, "", nil, [randomization.UuidSize]byte{}, "", nil, err
	}

	imageUuid, imageUuidStr, err := extractImageUUID(imageConnection)
	if err != nil {
		return nil, nil, "", nil, [randomization.UuidSize]byte{}, "", nil, err
	}

	if err := imageConnection.CleanClose(); err != nil {
		return nil, nil, "", nil, [randomization.UuidSize]byte{}, "", nil, fmt.Errorf("%w:\n%w", ErrArtifactImageConnectionClose, err)
	}

	return partUuidToFstabEntry, baseImageVerityMetadata, osRelease, osPackages, imageUuid, imageUuidStr, cosiBootMetadata, nil
}

func extractImageUUID(imageConnection *imageconnection.ImageConnection) ([randomization.UuidSize]byte, string, error) {
	var emptyUuid [randomization.UuidSize]byte

	releasePath := filepath.Join(imageConnection.Chroot().RootDir(), ImageCustomizerReleasePath)
	data, err := file.Read(releasePath)
	if err != nil {
		return emptyUuid, "", fmt.Errorf("%w (path='%s'):\n%w", ErrArtifactReleaseFileRead, releasePath, err)
	}

	lines := strings.Split(string(data), "\n")
	line, found := sliceutils.FindValueFunc(lines, func(line string) bool {
		return strings.HasPrefix(line, "IMAGE_UUID=")
	})
	if !found {
		return emptyUuid, "", fmt.Errorf("%w (path='%s')", ErrArtifactImageUuidNotFound, releasePath)
	}

	uuidStr := strings.Trim(strings.TrimPrefix(line, "IMAGE_UUID="), `"`)

	parsed, err := randomization.ParseUuidString(uuidStr)
	if err != nil {
		return emptyUuid, "", fmt.Errorf("%w (IMAGE_UUID='%s'):\n%w", ErrArtifactImageUuidParse, uuidStr, err)
	}

	return parsed, uuidStr, nil
}
