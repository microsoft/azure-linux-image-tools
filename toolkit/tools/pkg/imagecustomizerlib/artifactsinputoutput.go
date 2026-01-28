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

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/randomization"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/sliceutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
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
	previewFeatures []imagecustomizerapi.PreviewFeature,
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

		// Create subdirectory for UKIs
		ukiOutputSubdir := filepath.Join(outputDir, string(imagecustomizerapi.OutputArtifactsItemUkis))
		err = os.MkdirAll(ukiOutputSubdir, 0o755)
		if err != nil {
			return fmt.Errorf("failed to create ukis subdirectory:\n%w", err)
		}

		for _, entry := range dirEntries {
			if !entry.IsDir() && ukiRegex.MatchString(entry.Name()) {
				// Copy main UKI file
				srcPath := filepath.Join(ukiDir, entry.Name())
				destPath := filepath.Join(ukiOutputSubdir, entry.Name())
				err := file.Copy(srcPath, destPath)
				if err != nil {
					return fmt.Errorf("%w (source='%s', destination='%s'):\n%w", ErrArtifactBinaryCopy, srcPath, destPath, err)
				}

				source := "./" + string(imagecustomizerapi.OutputArtifactsItemUkis) + "/" + entry.Name()

				outputArtifactsMetadata = append(outputArtifactsMetadata, imagecustomizerapi.InjectArtifactMetadata{
					Partition:   espInjectFilePartition,
					Source:      source,
					Destination: filepath.Join("/", UkiOutputDir, entry.Name()),
					Type:        imagecustomizerapi.OutputArtifactsItemUkis,
				})
				logger.Log.Debugf("Added UKI file to metadata: %s", entry.Name())

				// Check for and copy UKI addon directory (e.g., vmlinuz-6.6.104.2-4.azl3.efi.extra.d/)
				addonDirName := entry.Name() + ".extra.d"
				addonSrcDir := filepath.Join(ukiDir, addonDirName)
				addonDirExists, err := file.DirExists(addonSrcDir)
				if err != nil {
					return fmt.Errorf("failed to check for UKI addon directory (%s):\n%w", addonSrcDir, err)
				}

				if addonDirExists {
					logger.Log.Debugf("Found UKI addon directory: %s", addonDirName)
					addonDestDir := filepath.Join(ukiOutputSubdir, addonDirName)

					// Copy the entire addon directory with all its contents
					err = file.CopyDir(addonSrcDir, addonDestDir, 0o755, 0o644, nil)
					if err != nil {
						return fmt.Errorf("failed to copy UKI addon directory (%s):\n%w", addonSrcDir, err)
					}

					addonEntries, err := os.ReadDir(addonSrcDir)
					if err != nil {
						return fmt.Errorf("failed to read UKI addon directory (%s):\n%w", addonSrcDir, err)
					}

					for _, addonEntry := range addonEntries {
						if !addonEntry.IsDir() {
							addonFileName := addonEntry.Name()
							addonSource := "./" + string(imagecustomizerapi.OutputArtifactsItemUkis) + "/" + addonDirName + "/" + addonFileName
							addonDestination := filepath.Join("/", UkiOutputDir, addonDirName, addonFileName)

							outputArtifactsMetadata = append(outputArtifactsMetadata, imagecustomizerapi.InjectArtifactMetadata{
								Partition:   espInjectFilePartition,
								Source:      addonSource,
								Destination: addonDestination,
								Type:        imagecustomizerapi.OutputArtifactsItemUkiAddons,
							})

							logger.Log.Debugf("Added UKI addon file to metadata: %s", addonFileName)
						}
					}
				} else {
					// No addon directory found - UKI with cmdline embedded in the main UKI file
					logger.Log.Debugf("No addon directory found for UKI: %s (single UKI file or no cmdline customization)", entry.Name())
				}
			}
		}
	}

	// Output shim
	if slices.Contains(items, imagecustomizerapi.OutputArtifactsItemShim) {
		// Create subdirectory for shim
		shimOutputSubdir := filepath.Join(outputDir, string(imagecustomizerapi.OutputArtifactsItemShim))
		err = os.MkdirAll(shimOutputSubdir, 0o755)
		if err != nil {
			return fmt.Errorf("failed to create shim subdirectory:\n%w", err)
		}

		srcPath := filepath.Join(systemBootPartitionTmpDir, ShimDir, bootConfig.bootBinary)
		destPath := filepath.Join(shimOutputSubdir, bootConfig.bootBinary)
		err := file.Copy(srcPath, destPath)
		if err != nil {
			return fmt.Errorf("%w (source='%s', destination='%s'):\n%w", ErrArtifactBinaryCopy, srcPath, destPath, err)
		}

		source := "./" + string(imagecustomizerapi.OutputArtifactsItemShim) + "/" + bootConfig.bootBinary

		outputArtifactsMetadata = append(outputArtifactsMetadata, imagecustomizerapi.InjectArtifactMetadata{
			Partition:   espInjectFilePartition,
			Source:      source,
			Destination: filepath.Join("/", ShimDir, bootConfig.bootBinary),
			Type:        imagecustomizerapi.OutputArtifactsItemShim,
		})
		logger.Log.Debugf("Added shim file to metadata: %s", bootConfig.bootBinary)
	}

	// Output systemd-boot
	if slices.Contains(items, imagecustomizerapi.OutputArtifactsItemSystemdBoot) {
		// Create subdirectory for systemd-boot
		systemdBootOutputSubdir := filepath.Join(outputDir, string(imagecustomizerapi.OutputArtifactsItemSystemdBoot))
		err = os.MkdirAll(systemdBootOutputSubdir, 0o755)
		if err != nil {
			return fmt.Errorf("failed to create systemd-boot subdirectory:\n%w", err)
		}

		srcPath := filepath.Join(systemBootPartitionTmpDir, SystemdBootDir, bootConfig.systemdBootBinary)
		destPath := filepath.Join(systemdBootOutputSubdir, bootConfig.systemdBootBinary)
		err := file.Copy(srcPath, destPath)
		if err != nil {
			return fmt.Errorf("%w (source='%s', destination='%s'):\n%w", ErrArtifactBinaryCopy, srcPath, destPath, err)
		}

		source := "./" + string(imagecustomizerapi.OutputArtifactsItemSystemdBoot) + "/" + bootConfig.systemdBootBinary

		outputArtifactsMetadata = append(outputArtifactsMetadata, imagecustomizerapi.InjectArtifactMetadata{
			Partition:   espInjectFilePartition,
			Source:      source,
			Destination: filepath.Join("/", SystemdBootDir, bootConfig.systemdBootBinary),
			Type:        imagecustomizerapi.OutputArtifactsItemSystemdBoot,
		})
		logger.Log.Debugf("Added systemd-boot file to metadata: %s", bootConfig.systemdBootBinary)
	}

	// Output verity hash
	if slices.Contains(items, imagecustomizerapi.OutputArtifactsItemVerityHash) {
		// Create subdirectory for verity hashes
		verityHashOutputSubdir := filepath.Join(outputDir, string(imagecustomizerapi.OutputArtifactsItemVerityHash))
		err = os.MkdirAll(verityHashOutputSubdir, 0o755)
		if err != nil {
			return fmt.Errorf("failed to create verity-hash subdirectory:\n%w", err)
		}

		for _, verity := range verityMetadata {
			if verity.hashSignaturePath == "" {
				continue
			}

			// Use the exact destination filename (e.g., "usr.hash" if hashSignaturePath is "/boot/usr.hash").
			destination := strings.TrimPrefix(verity.hashSignaturePath, "/boot")
			hashFile := filepath.Base(destination)
			destPath := filepath.Join(verityHashOutputSubdir, hashFile)
			err = file.Write(verity.rootHash, destPath)
			if err != nil {
				return fmt.Errorf("%w (name='%s', path='%s'):\n%w", ErrArtifactRootHashDump, verity.name, destPath, err)
			}

			source := "./" + string(imagecustomizerapi.OutputArtifactsItemVerityHash) + "/" + hashFile
			outputArtifactsMetadata = append(outputArtifactsMetadata, imagecustomizerapi.InjectArtifactMetadata{
				Partition:   bootInjectFilePartition,
				Source:      source,
				Destination: filepath.Join("/", destination),
				Type:        imagecustomizerapi.OutputArtifactsItemVerityHash,
			})
			logger.Log.Debugf("Added verity hash file to metadata: %s", hashFile)
		}
	}

	err = writeInjectFilesYaml(outputArtifactsMetadata, outputDir, previewFeatures)
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

func writeInjectFilesYaml(metadata []imagecustomizerapi.InjectArtifactMetadata, outputDir string,
	previewFeatures []imagecustomizerapi.PreviewFeature,
) error {
	injectPreviewFeatures := []imagecustomizerapi.PreviewFeature{imagecustomizerapi.PreviewFeatureInjectFiles}
	if slices.Contains(previewFeatures, imagecustomizerapi.PreviewFeatureCosiCompression) {
		injectPreviewFeatures = append(injectPreviewFeatures, imagecustomizerapi.PreviewFeatureCosiCompression)
	}

	yamlStruct := imagecustomizerapi.InjectFilesConfig{
		InjectFiles:     metadata,
		PreviewFeatures: injectPreviewFeatures,
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

func InjectFilesWithConfigFile(ctx context.Context, configFile string, options InjectFilesOptions) error {
	var config imagecustomizerapi.InjectFilesConfig
	err := imagecustomizerapi.UnmarshalYamlFile(configFile, &config)
	if err != nil {
		return err
	}

	if err := config.IsValid(); err != nil {
		return fmt.Errorf("%w:\n%w", ErrArtifactInvalidInjectFilesConfig, err)
	}

	if err := options.IsValid(); err != nil {
		return err
	}

	baseConfigPath, _ := filepath.Split(configFile)

	absBaseConfigPath, err := filepath.Abs(baseConfigPath)
	if err != nil {
		return fmt.Errorf("%w (path='%s'):\n%w", ErrArtifactInjectFilesPathResolution, baseConfigPath, err)
	}

	err = injectFilesWithOptions(ctx, absBaseConfigPath, config.InjectFiles, options, config.PreviewFeatures)
	if err != nil {
		return err
	}

	return nil
}

func injectFilesWithOptions(ctx context.Context, baseConfigPath string,
	metadata []imagecustomizerapi.InjectArtifactMetadata, options InjectFilesOptions,
	previewFeatures []imagecustomizerapi.PreviewFeature,
) error {
	logger.Log.Debugf("Injecting Files")

	ctx, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "inject_files")
	defer span.End()

	buildDirAbs, err := filepath.Abs(options.BuildDir)
	if err != nil {
		return err
	}
	rawImageFile := filepath.Join(buildDirAbs, BaseImageName)

	detectedImageFormat, err := convertImageToRaw(options.InputImageFile, rawImageFile)
	if err != nil {
		return err
	}

	err = options.verifyPreviewFeatures(previewFeatures)
	if err != nil {
		return err
	}

	if options.OutputImageFormat != "" {
		detectedImageFormat = imagecustomizerapi.ImageFormatType(options.OutputImageFormat)
	}

	outputImageFile := options.OutputImageFile
	if outputImageFile == "" {
		outputImageFile = options.InputImageFile
	}

	err = injectFilesIntoImage(options.BuildDir, baseConfigPath, rawImageFile, metadata)
	if err != nil {
		return err
	}

	err = exportImageForInjectFiles(ctx, buildDirAbs, rawImageFile, detectedImageFormat, outputImageFile,
		options.CosiCompressionLevel, previewFeatures)
	if err != nil {
		return err
	}

	logger.Log.Infof("Success!")

	return nil
}

func injectFilesIntoImage(buildDir string, baseConfigPath string, rawImageFile string,
	metadata []imagecustomizerapi.InjectArtifactMetadata,
) error {
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

			partition, _, err := findPartition(item.Partition.MountIdType, item.Partition.Id, diskPartitions)
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

	return nil
}

func exportImageForInjectFiles(ctx context.Context, buildDirAbs string, rawImageFile string,
	detectedImageFormat imagecustomizerapi.ImageFormatType, outputImageFile string, cosiCompressionLevel *int,
	previewFeatures []imagecustomizerapi.PreviewFeature,
) error {
	if detectedImageFormat == imagecustomizerapi.ImageFormatTypeCosi || detectedImageFormat == imagecustomizerapi.ImageFormatTypeBareMetalImage {
		partitionsLayout, baseImageVerityMetadata, osRelease, osPackages, imageUuid, imageUuidStr, cosiBootMetadata,
			readonlyPartUuids, err := prepareImageConversionData(ctx, rawImageFile, buildDirAbs, "imageroot", previewFeatures)
		if err != nil {
			return err
		}

		partitionOriginalSizes, err := shrinkFilesystemsHelper(ctx, rawImageFile, readonlyPartUuids)
		if err != nil {
			return fmt.Errorf("%w:\n%w", ErrShrinkFilesystems, err)
		}

		compressionLevel := defaultCosiCompressionLevel(detectedImageFormat)
		if cosiCompressionLevel != nil {
			compressionLevel = *cosiCompressionLevel
		}

		compressionLong := defaultCosiCompressionLong(detectedImageFormat)

		includeVhdFooter := detectedImageFormat == imagecustomizerapi.ImageFormatTypeBareMetalImage

		err = convertToCosi(buildDirAbs, rawImageFile, outputImageFile, partitionsLayout,
			baseImageVerityMetadata, osRelease, osPackages, imageUuid, imageUuidStr, cosiBootMetadata,
			compressionLevel, compressionLong, includeVhdFooter, partitionOriginalSizes)
		if err != nil {
			return fmt.Errorf("%w (output='%s'):\n%w", ErrArtifactCosiImageConversion, outputImageFile, err)
		}
	} else {
		err := ConvertImageFile(rawImageFile, outputImageFile, detectedImageFormat)
		if err != nil {
			return fmt.Errorf("%w (output='%s', format='%s'):\n%w", ErrArtifactOutputImageConversion, outputImageFile,
				detectedImageFormat, err)
		}
	}

	return nil
}

func prepareImageConversionData(ctx context.Context, rawImageFile string, buildDir string,
	chrootDir string, previewFeatures []imagecustomizerapi.PreviewFeature,
) ([]fstabEntryPartNum, []verityDeviceMetadata, string,
	[]OsPackage, [randomization.UuidSize]byte, string, *CosiBootloader, []string, error,
) {
	imageConnection, partitionsLayout, baseImageVerityMetadata, readonlyPartUuids, err := connectToExistingImage(
		ctx, rawImageFile, buildDir, chrootDir, true, true, true, true)
	if err != nil {
		err = fmt.Errorf("%w:\n%w", ErrArtifactImageConnectionForExtraction, err)
		return nil, nil, "", nil, [randomization.UuidSize]byte{}, "", nil, nil, err
	}
	defer imageConnection.Close()

	targetOs, err := targetos.GetInstalledTargetOs(imageConnection.Chroot().RootDir())
	if err != nil {
		return nil, nil, "", nil, [randomization.UuidSize]byte{}, "", nil, nil, fmt.Errorf("failed to determine target OS:\n%w", err)
	}

	err = validateDistroPreviewFeatures(targetOs, previewFeatures)
	if err != nil {
		return nil, nil, "", nil, [randomization.UuidSize]byte{}, "", nil, nil, err
	}

	osRelease, err := extractOSRelease(imageConnection)
	if err != nil {
		return nil, nil, "", nil, [randomization.UuidSize]byte{}, "", nil, nil, err
	}

	distroHandler, err := NewDistroHandlerFromChroot(imageConnection.Chroot())
	if err != nil {
		return nil, nil, "", nil, [randomization.UuidSize]byte{}, "", nil, nil, fmt.Errorf("failed to detect distribution:\n%w", err)
	}

	osPackages, cosiBootMetadata, err := collectOSInfoHelper(ctx, buildDir, imageConnection, distroHandler)
	if err != nil {
		return nil, nil, "", nil, [randomization.UuidSize]byte{}, "", nil, nil, err
	}

	imageUuid, imageUuidStr, err := extractImageUUID(imageConnection)
	if err != nil {
		return nil, nil, "", nil, [randomization.UuidSize]byte{}, "", nil, nil, err
	}

	if err := imageConnection.CleanClose(); err != nil {
		err = fmt.Errorf("%w:\n%w", ErrArtifactImageConnectionClose, err)
		return nil, nil, "", nil, [randomization.UuidSize]byte{}, "", nil, nil, err
	}

	return partitionsLayout, baseImageVerityMetadata, osRelease, osPackages, imageUuid, imageUuidStr,
		cosiBootMetadata, readonlyPartUuids, nil
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
