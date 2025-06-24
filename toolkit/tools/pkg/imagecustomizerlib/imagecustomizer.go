// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/osinfo"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/randomization"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safemount"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sys/unix"
)

const (
	tmpPartitionDirName     = "tmp-partition"
	tmpEspPartitionDirName  = "tmp-esp-partition"
	tmpBootPartitionDirName = "tmp-boot-partition"

	// qemu-specific formats
	QemuFormatVpc = "vpc"

	BaseImageName                = "image.raw"
	PartitionCustomizedImageName = "image2.raw"

	diskFreeWarnThresholdBytes   = 500 * diskutils.MiB
	diskFreeWarnThresholdPercent = 0.05
	toolsRootImageDir            = "_imageroot"
	toolsRoot                    = "toolsroot"

	OtelTracerName = "imagecustomizerlib"
)

// Version specifies the version of the Azure Linux Image Customizer tool.
// The value of this string is inserted during compilation via a linker flag.
var ToolVersion = ""

type ImageCustomizerParameters struct {
	// build dirs
	buildDirAbs string

	// input image
	inputImageFile   string
	inputImageFormat string
	inputIsIso       bool

	// configurations
	configPath            string
	config                *imagecustomizerapi.Config
	customizeOSPartitions bool
	useBaseImageRpmRepos  bool
	rpmsSources           []string
	packageSnapshotTime   string

	// intermediate writeable image
	rawImageFile string

	// output image
	outputImageFormat imagecustomizerapi.ImageFormatType
	outputIsIso       bool
	outputIsPxe       bool
	outputImageFile   string
	outputImageDir    string

	imageUuid    [randomization.UuidSize]byte
	imageUuidStr string

	baseImageVerityMetadata []verityDeviceMetadata
	verityMetadata          []verityDeviceMetadata

	partUuidToFstabEntry map[string]diskutils.FstabEntry
	osRelease            string
	osPackages           []OsPackage
}

type verityDeviceMetadata struct {
	name                  string
	rootHash              string
	dataPartUuid          string
	hashPartUuid          string
	dataDeviceMountIdType imagecustomizerapi.MountIdentifierType
	hashDeviceMountIdType imagecustomizerapi.MountIdentifierType
	corruptionOption      imagecustomizerapi.CorruptionOption
	hashSignaturePath     string
}

func createImageCustomizerParameters(buildDir string,
	inputImageFile string,
	configPath string, config *imagecustomizerapi.Config,
	useBaseImageRpmRepos bool, rpmsSources []string,
	outputImageFormat string, outputImageFile string, packageSnapshotTime string,
) (*ImageCustomizerParameters, error) {
	ic := &ImageCustomizerParameters{}

	// working directories
	buildDirAbs, err := filepath.Abs(buildDir)
	if err != nil {
		return nil, err
	}

	ic.buildDirAbs = buildDirAbs

	// input image
	ic.inputImageFile = inputImageFile
	if ic.inputImageFile == "" && config.Input.Image.Path != "" {
		ic.inputImageFile = file.GetAbsPathWithBase(configPath, config.Input.Image.Path)
	}

	ic.inputImageFormat = strings.TrimLeft(filepath.Ext(ic.inputImageFile), ".")
	ic.inputIsIso = ic.inputImageFormat == string(imagecustomizerapi.ImageFormatTypeIso)

	// Create a uuid for the image
	imageUuid, imageUuidStr, err := randomization.CreateUuid()
	if err != nil {
		return nil, err
	}
	ic.imageUuid = imageUuid
	ic.imageUuidStr = imageUuidStr

	// configuration
	ic.configPath = configPath
	ic.config = config
	ic.customizeOSPartitions = config.CustomizePartitions() || config.OS != nil ||
		len(config.Scripts.PostCustomization) > 0 ||
		len(config.Scripts.FinalizeCustomization) > 0

	ic.useBaseImageRpmRepos = useBaseImageRpmRepos
	ic.rpmsSources = rpmsSources

	err = ValidateRpmSources(rpmsSources)
	if err != nil {
		return nil, err
	}

	// intermediate writeable image
	ic.rawImageFile = filepath.Join(buildDirAbs, BaseImageName)

	// output image
	ic.outputImageFormat = imagecustomizerapi.ImageFormatType(outputImageFormat)
	if err := ic.outputImageFormat.IsValid(); err != nil {
		return nil, fmt.Errorf("invalid output image format:\n%w", err)
	}

	if ic.outputImageFormat == "" {
		ic.outputImageFormat = config.Output.Image.Format
	}

	ic.outputImageFile = outputImageFile
	if ic.outputImageFile == "" && config.Output.Image.Path != "" {
		ic.outputImageFile = file.GetAbsPathWithBase(configPath, config.Output.Image.Path)
	}

	ic.outputImageDir = filepath.Dir(ic.outputImageFile)
	ic.outputIsIso = ic.outputImageFormat == imagecustomizerapi.ImageFormatTypeIso
	ic.outputIsPxe = ic.outputImageFormat == imagecustomizerapi.ImageFormatTypePxeDir ||
		ic.outputImageFormat == imagecustomizerapi.ImageFormatTypePxeTar

	if ic.inputIsIso {

		// While re-creating a disk image from the iso is technically possible,
		// we are choosing to not implement it until there is a need.
		if !ic.outputIsIso && !ic.outputIsPxe {
			return nil, fmt.Errorf("cannot generate output format (%s) from the given input format (%s)", ic.outputImageFormat, ic.inputImageFormat)
		}

		// While defining a storage configuration can work when the input image is
		// an iso, there is no obvious point of moving content between partitions
		// where all partitions get collapsed into the squashfs at the end.
		if config.CustomizePartitions() {
			return nil, fmt.Errorf("cannot customize partitions when the input is an iso")
		}
	}

	ic.packageSnapshotTime = packageSnapshotTime

	return ic, nil
}

func CustomizeImageWithConfigFile(buildDir string, configFile string, inputImageFile string,
	rpmsSources []string, outputImageFile string, outputImageFormat string,
	useBaseImageRpmRepos bool, packageSnapshotTime string,
) error {
	var err error

	var config imagecustomizerapi.Config

	err = imagecustomizerapi.UnmarshalYamlFile(configFile, &config)
	if err != nil {
		return err
	}

	baseConfigPath, _ := filepath.Split(configFile)

	absBaseConfigPath, err := filepath.Abs(baseConfigPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path of config file directory:\n%w", err)
	}

	err = CustomizeImage(buildDir, absBaseConfigPath, &config, inputImageFile, rpmsSources, outputImageFile, outputImageFormat,
		useBaseImageRpmRepos, packageSnapshotTime)
	if err != nil {
		return err
	}

	return nil
}

func cleanUp(ic *ImageCustomizerParameters) error {
	err := file.RemoveFileIfExists(ic.rawImageFile)
	if err != nil {
		return err
	}

	return nil
}

func CustomizeImage(buildDir string, baseConfigPath string, config *imagecustomizerapi.Config, inputImageFile string,
	rpmsSources []string, outputImageFile string, outputImageFormat string,
	useBaseImageRpmRepos bool, packageSnapshotTime string,
) error {
	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(context.Background(), "CustomizeImage")
	span.SetAttributes(
		attribute.String("outputImageFormat", string(outputImageFormat)),
	)
	defer span.End()

	err := ValidateConfig(baseConfigPath, config, inputImageFile, rpmsSources, outputImageFile, outputImageFormat, useBaseImageRpmRepos, packageSnapshotTime, false)
	if err != nil {
		return fmt.Errorf("invalid image config:\n%w", err)
	}

	imageCustomizerParameters, err := createImageCustomizerParameters(buildDir, inputImageFile,
		baseConfigPath, config, useBaseImageRpmRepos, rpmsSources,
		outputImageFormat, outputImageFile, packageSnapshotTime)
	if err != nil {
		return fmt.Errorf("invalid parameters:\n%w", err)
	}
	defer func() {
		cleanupErr := cleanUp(imageCustomizerParameters)
		if cleanupErr != nil {
			if err != nil {
				err = fmt.Errorf("%w:\nfailed to clean-up:\n%w", err, cleanupErr)
			} else {
				err = fmt.Errorf("failed to clean-up:\n%w", cleanupErr)
			}
		}
	}()

	err = CheckEnvironmentVars()
	if err != nil {
		return err
	}

	LogVersionsOfToolDeps()

	// ensure build and output folders are created up front
	err = os.MkdirAll(imageCustomizerParameters.buildDirAbs, os.ModePerm)
	if err != nil {
		return err
	}

	err = os.MkdirAll(imageCustomizerParameters.outputImageDir, os.ModePerm)
	if err != nil {
		return err
	}

	inputIsoArtifacts, err := convertInputImageToWriteableFormat(imageCustomizerParameters)
	if err != nil {
		return fmt.Errorf("failed to convert input image to a raw image:\n%w", err)
	}
	defer func() {
		if inputIsoArtifacts != nil {
			cleanupErr := inputIsoArtifacts.cleanUp()
			if cleanupErr != nil {
				if err != nil {
					err = fmt.Errorf("%w:\nfailed to clean-up iso builder state:\n%w", err, cleanupErr)
				} else {
					err = fmt.Errorf("failed to clean-up iso builder state:\n%w", cleanupErr)
				}
			}
		}
	}()

	err = customizeOSContents(imageCustomizerParameters)
	if err != nil {
		return fmt.Errorf("failed to customize raw image:\n%w", err)
	}

	if config.Output.Artifacts != nil {
		outputDir := file.GetAbsPathWithBase(baseConfigPath, config.Output.Artifacts.Path)

		err = outputArtifacts(config.Output.Artifacts.Items, outputDir, imageCustomizerParameters.buildDirAbs,
			imageCustomizerParameters.rawImageFile, imageCustomizerParameters.verityMetadata)
		if err != nil {
			return err
		}
	}

	err = convertWriteableFormatToOutputImage(imageCustomizerParameters, inputIsoArtifacts)
	if err != nil {
		return fmt.Errorf("failed to convert customized raw image to output format:\n%w", err)
	}

	logger.Log.Infof("Success!")

	return nil
}

func convertInputImageToWriteableFormat(ic *ImageCustomizerParameters) (*IsoArtifactsStore, error) {
	logger.Log.Infof("Converting input image to a writeable format")

	if ic.inputIsIso {

		inputIsoArtifacts, err := createIsoArtifactStoreFromIsoImage(ic.inputImageFile, filepath.Join(ic.buildDirAbs, "from-iso"))
		if err != nil {
			return inputIsoArtifacts, fmt.Errorf("failed to create artifacts store from (%s):\n%w", ic.inputImageFile, err)
		}

		_, convertInitramfsType, err := buildLiveOSConfig(inputIsoArtifacts, ic.config.Iso,
			ic.config.Pxe, ic.outputImageFormat)
		if err != nil {
			return nil, fmt.Errorf("failed to build Live OS configuration:\n%w", err)
		}

		// If the input is a LiveOS iso and there are OS customizations
		// defined, we create a writeable disk image so that mic can modify
		// it. If no OS customizations are defined, we can skip this step and
		// just re-use the existing squashfs.
		rebuildFullOsImage := ic.customizeOSPartitions || convertInitramfsType

		if rebuildFullOsImage {
			err = createWriteableImageFromArtifacts(ic.buildDirAbs, inputIsoArtifacts, ic.rawImageFile)
			if err != nil {
				return nil, fmt.Errorf("failed to create writeable image:\n%w", err)
			}
		}

		return inputIsoArtifacts, nil
	} else {
		logger.Log.Infof("Creating raw base image: %s", ic.rawImageFile)

		_, err := convertImageToRaw(ic.inputImageFile, ic.inputImageFormat, ic.rawImageFile)
		if err != nil {
			return nil, err
		}

		return nil, nil
	}
}

func convertImageToRaw(inputImageFile string, inputImageFormat string,
	rawImageFile string,
) (imagecustomizerapi.ImageFormatType, error) {
	imageInfo, err := GetImageFileInfo(inputImageFile)
	if err != nil {
		return "", fmt.Errorf("failed to detect input image (%s) format:\n%w", inputImageFile, err)
	}

	detectedImageFormat := imageInfo.Format
	sourceArg := fmt.Sprintf("file.filename=%s", qemuImgEscapeOptionValue(inputImageFile))

	// The fixed-size VHD format is just a raw disk file with small metadata footer appended to the end. Unfortunatley,
	// that footer doesn't contain a file signature (i.e. "magic number"). So, qemu-img can't correctly detect this
	// format and instead reports fixed-size VHDs as raw images. So, use the filename extension as a hint.
	if inputImageFormat == "vhd" && detectedImageFormat == "raw" {
		// Force qemu-img to treat the file as a VHD.
		detectedImageFormat = "vpc"
	}

	if detectedImageFormat == "vpc" {
		// There are actually two different ways of calculating the disk size of a VHD file. The old method, which is
		// used by Microsoft Virtual PC, uses the VHD's footer's "Disk Geometry" (cylinder, heads, and sectors per
		// track/cylinder) fields. Whereas, the new method, which is used by Hyper-V, simply uses the VHD's footer's
		// "Current Size" field. The qemu-img tool does try to correctly detect which one is being used by looking at
		// the footer's "Creator Application" field. But if the tool that created the VHD uses a name that qemu-img
		// doesn't recognize, then the heuristic can pick the wrong one. This seems to be the case for VHDs downloaded
		// from Azure. For the Image Customizer tool, it is pretty safe to assume all VHDs use the Hyper-V format.
		// So, force qemu-img to use that format.
		sourceArg += ",driver=vpc,force_size_calc=current_size"
	}

	err = shell.ExecuteLiveWithErr(1, "qemu-img", "convert", "-O", "raw", "--image-opts", sourceArg, rawImageFile)
	if err != nil {
		return "", fmt.Errorf("failed to convert image file to raw format:\n%w", err)
	}

	format, err := qemuStringtoImageFormatType(detectedImageFormat)
	if err != nil {
		return "", err
	}
	return format, nil
}

func qemuStringtoImageFormatType(qemuFormat string) (imagecustomizerapi.ImageFormatType, error) {
	switch qemuFormat {
	case "raw":
		return imagecustomizerapi.ImageFormatTypeRaw, nil
	case "qcow2":
		return imagecustomizerapi.ImageFormatTypeQcow2, nil
	case "vpc":
		return imagecustomizerapi.ImageFormatTypeVhd, nil
	case "vhdx":
		return imagecustomizerapi.ImageFormatTypeVhdx, nil
	case "iso":
		return imagecustomizerapi.ImageFormatTypeIso, nil
	default:
		return "", fmt.Errorf("unsupported qemu-img format: %s", qemuFormat)
	}
}

func qemuImgEscapeOptionValue(value string) string {
	// Commas are escaped by doubling them up.
	return strings.ReplaceAll(value, ",", ",,")
}

func customizeOSContents(ic *ImageCustomizerParameters) error {
	// If there are OS customizations, then we proceed as usual.
	// If there are no OS customizations, and the input is an iso, we just
	// return because this function is mainly about OS customizations.
	// This function also supports shrinking/exporting partitions. While
	// we could support those functions for input isos, we are choosing to
	// not support them until there is an actual need/a future time.
	// We explicitly inform the user of the lack of support earlier during
	// mic parameter validation (see createImageCustomizerParameters()).
	if !ic.customizeOSPartitions && ic.inputIsIso {
		return nil
	}

	// The code beyond this point assumes the OS object is always present. To
	// change the code to check before every usage whether the OS object is
	// present or not will lead to a messy mix of if statements that do not
	// serve the readibility of the code. A simpler solution is to instantiate
	// a default imagecustomizerapi.OS object if the passed in one is absent.
	// Then the code afterwards knows how to handle the default values
	// correctly, and thus it eliminates the need for many if statements.
	if ic.config.OS == nil {
		ic.config.OS = &imagecustomizerapi.OS{}
	}

	// Customize the partitions.
	partitionsCustomized, newRawImageFile, partIdToPartUuid, err := customizePartitions(ic.buildDirAbs,
		ic.configPath, ic.config, ic.rawImageFile)
	if err != nil {
		return err
	}

	if ic.rawImageFile != newRawImageFile {
		os.Remove(ic.rawImageFile)
		ic.rawImageFile = newRawImageFile
	}

	// Customize the raw image file.
	partUuidToFstabEntry, baseImageVerityMetadata, osRelease, osPackages, err := customizeImageHelper(ic.buildDirAbs, ic.configPath,
		ic.config, ic.rawImageFile, ic.rpmsSources, ic.useBaseImageRpmRepos, partitionsCustomized, ic.imageUuidStr, ic.packageSnapshotTime,
		ic.outputImageFormat)
	if err != nil {
		return err
	}

	if len(baseImageVerityMetadata) > 0 {
		previewFeatureEnabled := slices.Contains(ic.config.PreviewFeatures,
			imagecustomizerapi.PreviewFeatureReinitializeVerity)
		if !previewFeatureEnabled {
			return fmt.Errorf("Please enable the '%s' preview feature to customize a verity enabled base image",
				imagecustomizerapi.PreviewFeatureReinitializeVerity)
		}
	}

	ic.partUuidToFstabEntry = partUuidToFstabEntry
	ic.baseImageVerityMetadata = baseImageVerityMetadata
	ic.osRelease = osRelease
	ic.osPackages = osPackages

	// For COSI, always shrink the filesystems.
	shrinkPartitions := ic.outputImageFormat == imagecustomizerapi.ImageFormatTypeCosi
	if shrinkPartitions {
		err = shrinkFilesystemsHelper(ic.rawImageFile)
		if err != nil {
			return fmt.Errorf("failed to shrink filesystems:\n%w", err)
		}
	}

	if len(ic.config.Storage.Verity) > 0 || len(ic.baseImageVerityMetadata) > 0 {
		// Customize image for dm-verity, setting up verity metadata and security features.
		verityMetadata, err := customizeVerityImageHelper(ic.buildDirAbs, ic.config, ic.rawImageFile, partIdToPartUuid,
			shrinkPartitions, ic.baseImageVerityMetadata)
		if err != nil {
			return err
		}

		ic.verityMetadata = verityMetadata
	}

	if ic.config.OS.Uki != nil {
		err = createUki(ic.config.OS.Uki, ic.buildDirAbs, ic.rawImageFile)
		if err != nil {
			return err
		}
	}

	// Check file systems for corruption.
	err = checkFileSystems(ic.rawImageFile)
	if err != nil {
		return fmt.Errorf("failed to check filesystems:\n%w", err)
	}

	return nil
}

func convertWriteableFormatToOutputImage(ic *ImageCustomizerParameters, inputIsoArtifacts *IsoArtifactsStore) error {
	logger.Log.Infof("Converting customized OS partitions into the final image")

	// Create final output image file if requested.
	switch ic.outputImageFormat {
	case imagecustomizerapi.ImageFormatTypeVhd, imagecustomizerapi.ImageFormatVhdTypeFixed,
		imagecustomizerapi.ImageFormatTypeVhdx, imagecustomizerapi.ImageFormatTypeQcow2,
		imagecustomizerapi.ImageFormatTypeRaw:
		logger.Log.Infof("Writing: %s", ic.outputImageFile)

		err := ConvertImageFile(ic.rawImageFile, ic.outputImageFile, ic.outputImageFormat)
		if err != nil {
			return err
		}

	case imagecustomizerapi.ImageFormatTypeCosi:
		err := convertToCosi(ic.buildDirAbs, ic.rawImageFile, ic.outputImageFile, ic.partUuidToFstabEntry,
			ic.verityMetadata, ic.osRelease, ic.osPackages, ic.imageUuid, ic.imageUuidStr)
		if err != nil {
			return err
		}

	case imagecustomizerapi.ImageFormatTypeIso, imagecustomizerapi.ImageFormatTypePxeDir, imagecustomizerapi.ImageFormatTypePxeTar:
		// Decide whether we need to re-build the full OS image or not
		convertInitramfsType := false
		if inputIsoArtifacts != nil {
			// Let's check if use is converting from full os initramfs to bootstrap initramfs
			var err error
			_, convertInitramfsType, err = buildLiveOSConfig(inputIsoArtifacts, ic.config.Iso, ic.config.Pxe,
				ic.outputImageFormat)
			if err != nil {
				return fmt.Errorf("failed to build Live OS configuration\n%w", err)
			}
		}

		rebuildFullOsImage := ic.customizeOSPartitions || inputIsoArtifacts == nil || convertInitramfsType

		// Either re-build the full OS image, or just re-package the existing one
		if rebuildFullOsImage {
			requestedSELinuxMode := imagecustomizerapi.SELinuxModeDefault
			if ic.config.OS != nil {
				requestedSELinuxMode = ic.config.OS.SELinux.Mode
			}
			err := createLiveOSFromRaw(ic.buildDirAbs, ic.configPath, inputIsoArtifacts, requestedSELinuxMode,
				ic.config.Iso, ic.config.Pxe, ic.rawImageFile, ic.outputImageFormat, ic.outputImageFile)
			if err != nil {
				return fmt.Errorf("failed to create Live OS artifacts:\n%w", err)
			}
		} else {
			err := repackageLiveOS(ic.buildDirAbs, ic.configPath, ic.config.Iso, ic.config.Pxe,
				inputIsoArtifacts, ic.outputImageFormat, ic.outputImageFile)
			if err != nil {
				return fmt.Errorf("failed to create Live OS artifacts:\n%w", err)
			}
		}
	}

	return nil
}

func ConvertImageFile(inputPath string, outputPath string, format imagecustomizerapi.ImageFormatType) error {
	qemuImageFormat, qemuOptions := toQemuImageFormat(format)

	qemuImgArgs := []string{"convert", "-O", qemuImageFormat}
	if qemuOptions != "" {
		qemuImgArgs = append(qemuImgArgs, "-o", qemuOptions)
	}
	qemuImgArgs = append(qemuImgArgs, inputPath, outputPath)

	err := shell.ExecuteLiveWithErr(1, "qemu-img", qemuImgArgs...)
	if err != nil {
		return fmt.Errorf("failed to convert image file to format: %s:\n%w", format, err)
	}

	return nil
}

func toQemuImageFormat(imageFormat imagecustomizerapi.ImageFormatType) (string, string) {
	switch imageFormat {
	case imagecustomizerapi.ImageFormatTypeVhd:
		// Use "force_size=on" to ensure the Hyper-V's VHD format is used instead of the old Microsoft Virtual PC's VHD
		// format.
		return QemuFormatVpc, "subformat=dynamic,force_size=on"

	case imagecustomizerapi.ImageFormatVhdTypeFixed:
		return QemuFormatVpc, "subformat=fixed,force_size=on"

	case imagecustomizerapi.ImageFormatTypeVhdx:
		// For VHDX, qemu-img dynamically picks the block-size based on the size of the disk.
		// However, this can result in a significantly larger file size than other formats.
		// So, use a fixed block-size of 2 MiB to match the block-sizes used for qcow2 and VHD.
		return string(imagecustomizerapi.ImageFormatTypeVhdx), "block_size=2097152"

	default:
		return string(imageFormat), ""
	}
}

func ValidateConfig(baseConfigPath string, config *imagecustomizerapi.Config, inputImageFile string, rpmsSources []string,
	outputImageFile, outputImageFormat string, useBaseImageRpmRepos bool, packageSnapshotTime string, newImage bool,
) error {
	err := config.IsValid()
	if err != nil {
		return err
	}

	if !newImage {
		err = validateInput(baseConfigPath, config.Input, inputImageFile)
		if err != nil {
			return err
		}
	}

	err = validateIsoConfig(baseConfigPath, config.Iso)
	if err != nil {
		return err
	}

	err = validateOsConfig(baseConfigPath, config.OS, rpmsSources, useBaseImageRpmRepos)
	if err != nil {
		return err
	}

	err = validateScripts(baseConfigPath, &config.Scripts)
	if err != nil {
		return err
	}

	err = validateOutput(baseConfigPath, config.Output, outputImageFile, outputImageFormat)
	if err != nil {
		return err
	}

	if err := validateSnapshotTimeInput(packageSnapshotTime, config.PreviewFeatures); err != nil {
		return err
	}

	return nil
}

func validateInput(baseConfigPath string, input imagecustomizerapi.Input, inputImageFile string) error {
	if inputImageFile == "" && input.Image.Path == "" {
		return fmt.Errorf("input image file must be specified, either via the command line option '--image-file' or in the config file property 'input.image.path'")
	}

	if inputImageFile != "" {
		if yes, err := file.IsFile(inputImageFile); err != nil {
			return fmt.Errorf("invalid command-line option '--image-file': '%s'\n%w", inputImageFile, err)
		} else if !yes {
			return fmt.Errorf("invalid command-line option '--image-file': '%s'\nnot a file", inputImageFile)
		}
	} else {
		inputImageAbsPath := file.GetAbsPathWithBase(baseConfigPath, input.Image.Path)
		if yes, err := file.IsFile(inputImageAbsPath); err != nil {
			return fmt.Errorf("invalid config file property 'input.image.path': '%s'\n%w", input.Image.Path, err)
		} else if !yes {
			return fmt.Errorf("invalid config file property 'input.image.path': '%s'\nnot a file", input.Image.Path)
		}
	}

	return nil
}

func validateAdditionalFiles(baseConfigPath string, additionalFiles imagecustomizerapi.AdditionalFileList) error {
	errs := []error(nil)
	for _, additionalFile := range additionalFiles {
		switch {
		case additionalFile.Source != "":
			sourceFileFullPath := file.GetAbsPathWithBase(baseConfigPath, additionalFile.Source)
			isFile, err := file.IsFile(sourceFileFullPath)
			if err != nil {
				errs = append(errs, fmt.Errorf("invalid additionalFiles source file (%s):\n%w", additionalFile.Source, err))
			}

			if !isFile {
				errs = append(errs, fmt.Errorf("invalid additionalFiles source file (%s):\nnot a file",
					additionalFile.Source))
			}
		}
	}

	return errors.Join(errs...)
}

func validateIsoConfig(baseConfigPath string, config *imagecustomizerapi.Iso) error {
	if config == nil {
		return nil
	}

	err := validateAdditionalFiles(baseConfigPath, config.AdditionalFiles)
	if err != nil {
		return err
	}

	return nil
}

func validateOsConfig(baseConfigPath string, config *imagecustomizerapi.OS,
	rpmsSources []string, useBaseImageRpmRepos bool,
) error {
	if config == nil {
		return nil
	}

	var err error

	err = validatePackageLists(baseConfigPath, config, rpmsSources, useBaseImageRpmRepos)
	if err != nil {
		return err
	}

	err = validateAdditionalFiles(baseConfigPath, config.AdditionalFiles)
	if err != nil {
		return err
	}

	err = validateUsers(baseConfigPath, config.Users)
	if err != nil {
		return err
	}

	return nil
}

func validateScripts(baseConfigPath string, scripts *imagecustomizerapi.Scripts) error {
	if scripts == nil {
		return nil
	}

	for i, script := range scripts.PostCustomization {
		err := validateScript(baseConfigPath, &script)
		if err != nil {
			return fmt.Errorf("invalid postCustomization item at index %d:\n%w", i, err)
		}
	}

	for i, script := range scripts.FinalizeCustomization {
		err := validateScript(baseConfigPath, &script)
		if err != nil {
			return fmt.Errorf("invalid finalizeCustomization item at index %d:\n%w", i, err)
		}
	}

	return nil
}

func validateScript(baseConfigPath string, script *imagecustomizerapi.Script) error {
	if script.Path != "" {
		// Ensure that install scripts sit under the config file's parent directory.
		// This allows the install script to be run in the chroot environment by bind mounting the config directory.
		if !filepath.IsLocal(script.Path) {
			return fmt.Errorf("script file (%s) is not under config directory (%s)", script.Path, baseConfigPath)
		}

		fullPath := filepath.Join(baseConfigPath, script.Path)

		// Verify that the file exists.
		_, err := os.Stat(fullPath)
		if err != nil {
			return fmt.Errorf("couldn't read script file (%s):\n%w", script.Path, err)
		}
	}

	return nil
}

func validatePackageLists(baseConfigPath string, config *imagecustomizerapi.OS, rpmsSources []string,
	useBaseImageRpmRepos bool,
) error {
	if config == nil {
		return nil
	}

	allPackagesRemove, err := collectPackagesList(baseConfigPath, config.Packages.RemoveLists, config.Packages.Remove)
	if err != nil {
		return err
	}

	allPackagesInstall, err := collectPackagesList(baseConfigPath, config.Packages.InstallLists, config.Packages.Install)
	if err != nil {
		return err
	}

	allPackagesUpdate, err := collectPackagesList(baseConfigPath, config.Packages.UpdateLists, config.Packages.Update)
	if err != nil {
		return err
	}

	hasRpmSources := len(rpmsSources) > 0 || useBaseImageRpmRepos

	if !hasRpmSources {
		needRpmsSources := len(allPackagesInstall) > 0 || len(allPackagesUpdate) > 0 ||
			config.Packages.UpdateExistingPackages

		if needRpmsSources {
			return fmt.Errorf("have packages to install or update but no RPM sources were specified")
		}
	}

	config.Packages.Remove = allPackagesRemove
	config.Packages.Install = allPackagesInstall
	config.Packages.Update = allPackagesUpdate

	config.Packages.RemoveLists = nil
	config.Packages.InstallLists = nil
	config.Packages.UpdateLists = nil

	return nil
}

func validateOutput(baseConfigPath string, output imagecustomizerapi.Output, outputImageFile, outputImageFormat string) error {
	if outputImageFile == "" && output.Image.Path == "" {
		return fmt.Errorf("output image file must be specified, either via the command line option '--output-image-file' or in the config file property 'output.image.path'")
	}

	// Pxe output format allows the output to be a path.
	if output.Image.Format != imagecustomizerapi.ImageFormatTypePxeDir && outputImageFormat != string(imagecustomizerapi.ImageFormatTypePxeDir) {
		if outputImageFile != "" {
			if isDir, err := file.DirExists(outputImageFile); err != nil {
				return fmt.Errorf("invalid command-line option '--output-image-file': '%s'\n%w", outputImageFile, err)
			} else if isDir {
				return fmt.Errorf("invalid command-line option '--output-image-file': '%s'\nis a directory", outputImageFile)
			}
		} else {
			outputImageAbsPath := file.GetAbsPathWithBase(baseConfigPath, output.Image.Path)
			if isDir, err := file.DirExists(outputImageAbsPath); err != nil {
				return fmt.Errorf("invalid config file property 'output.image.path': '%s'\n%w", output.Image.Path, err)
			} else if isDir {
				return fmt.Errorf("invalid config file property 'output.image.path': '%s'\nis a directory", output.Image.Path)
			}
		}
	}

	if outputImageFormat == "" && output.Image.Format == imagecustomizerapi.ImageFormatTypeNone {
		return fmt.Errorf("output image format must be specified, either via the command line option '--output-image-format' or in the config file property 'output.image.format'")
	}

	return nil
}

func validateUsers(baseConfigPath string, users []imagecustomizerapi.User) error {
	for _, user := range users {
		err := validateUser(baseConfigPath, user)
		if err != nil {
			return fmt.Errorf("invalid user (%s):\n%w", user.Name, err)
		}
	}

	return nil
}

func validateUser(baseConfigPath string, user imagecustomizerapi.User) error {
	for _, path := range user.SSHPublicKeyPaths {
		absPath := file.GetAbsPathWithBase(baseConfigPath, path)
		isFile, err := file.IsFile(absPath)
		if err != nil {
			return fmt.Errorf("failed to find SSH public key file (%s):\n%w", path, err)
		}
		if !isFile {
			return fmt.Errorf("SSH public key path is not a file (%s)", path)
		}
	}

	return nil
}

func customizeImageHelper(buildDir string, baseConfigPath string, config *imagecustomizerapi.Config,
	rawImageFile string, rpmsSources []string, useBaseImageRpmRepos bool, partitionsCustomized bool,
	imageUuidStr string, packageSnapshotTime string, outputImageFormatType imagecustomizerapi.ImageFormatType,
) (map[string]diskutils.FstabEntry, []verityDeviceMetadata, string, []OsPackage, error) {
	logger.Log.Debugf("Customizing OS")

	imageConnection, partUuidToFstabEntry, baseImageVerityMetadata, err := connectToExistingImage(rawImageFile,
		buildDir, "imageroot", true, false)
	if err != nil {
		return nil, nil, "", nil, err
	}
	defer imageConnection.Close()

	osRelease, err := extractOSRelease(imageConnection)
	if err != nil {
		return nil, nil, "", nil, err
	}

	imageConnection.Chroot().UnsafeRun(func() error {
		distro, version := osinfo.GetDistroAndVersion()
		logger.Log.Infof("Base OS distro: %s", distro)
		logger.Log.Infof("Base OS version: %s", version)
		return nil
	})

	err = validateVerityMountPaths(imageConnection, config, partUuidToFstabEntry, baseImageVerityMetadata)
	if err != nil {
		return nil, nil, "", nil, fmt.Errorf("verity validation failed:\n%w", err)
	}

	// Do the actual customizations.
	err = doOsCustomizations(buildDir, baseConfigPath, config, imageConnection, rpmsSources,
		useBaseImageRpmRepos, partitionsCustomized, imageUuidStr, partUuidToFstabEntry, packageSnapshotTime)

	// collect OS info if generating a COSI image
	var osPackages []OsPackage
	if config.Output.Image.Format == imagecustomizerapi.ImageFormatTypeCosi || outputImageFormatType == imagecustomizerapi.ImageFormatTypeCosi {
		osPackages, err = collectOSInfo(imageConnection)
		if err != nil {
			return nil, nil, "", nil, err
		}
	}

	// Out of disk space errors can be difficult to diagnose.
	// So, warn about any partitions with low free space.
	warnOnLowFreeSpace(buildDir, imageConnection)

	if err != nil {
		return nil, nil, "", nil, err
	}

	err = imageConnection.CleanClose()
	if err != nil {
		return nil, nil, "", nil, err
	}

	return partUuidToFstabEntry, baseImageVerityMetadata, osRelease, osPackages, nil
}

func collectOSInfo(imageConnection *ImageConnection) ([]OsPackage, error) {
	osPackages, err := getAllPackagesFromChroot(imageConnection)
	if err != nil {
		return nil, fmt.Errorf("failed to extract installed packages:\n%w", err)
	}

	return osPackages, nil
}

func shrinkFilesystemsHelper(buildImageFile string) error {
	imageLoopback, err := safeloopback.NewLoopback(buildImageFile)
	if err != nil {
		return err
	}
	defer imageLoopback.Close()

	// Shrink the filesystems.
	err = shrinkFilesystems(imageLoopback.DevicePath())
	if err != nil {
		return err
	}

	err = imageLoopback.CleanClose()
	if err != nil {
		return err
	}

	return nil
}

func customizeVerityImageHelper(buildDir string, config *imagecustomizerapi.Config,
	buildImageFile string, partIdToPartUuid map[string]string, shrinkHashPartition bool,
	baseImageVerity []verityDeviceMetadata,
) ([]verityDeviceMetadata, error) {
	logger.Log.Infof("Provisioning verity")

	verityMetadata := []verityDeviceMetadata(nil)

	loopback, err := safeloopback.NewLoopback(buildImageFile)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to image file to provision verity:\n%w", err)
	}
	defer loopback.Close()

	diskPartitions, err := diskutils.GetDiskPartitions(loopback.DevicePath())
	if err != nil {
		return nil, err
	}

	sectorSize, _, err := diskutils.GetSectorSize(loopback.DevicePath())
	if err != nil {
		return nil, fmt.Errorf("failed to get disk's (%s) sector size:\n%w", loopback.DevicePath(), err)
	}

	for _, metadata := range baseImageVerity {
		// Find partitions.
		dataPartition, _, err := findPartitionHelper(imagecustomizerapi.MountIdentifierTypePartUuid,
			metadata.dataPartUuid, diskPartitions)
		if err != nil {
			return nil, fmt.Errorf("failed to find verity (%s) data partition:\n%w", metadata.name, err)
		}

		hashPartition, _, err := findPartitionHelper(imagecustomizerapi.MountIdentifierTypePartUuid,
			metadata.hashPartUuid, diskPartitions)
		if err != nil {
			return nil, fmt.Errorf("failed to find verity (%s) data partition:\n%w", metadata.name, err)
		}

		// Format hash partition.
		rootHash, err := verityFormat(loopback.DevicePath(), dataPartition.Path, hashPartition.Path,
			shrinkHashPartition, sectorSize)
		if err != nil {
			return nil, err
		}

		newMetadata := metadata
		newMetadata.rootHash = rootHash
		verityMetadata = append(verityMetadata, newMetadata)
	}

	for _, verityConfig := range config.Storage.Verity {
		// Extract the partition block device path.
		dataPartition, err := verityIdToPartition(verityConfig.DataDeviceId, verityConfig.DataDevice, partIdToPartUuid,
			diskPartitions)
		if err != nil {
			return nil, fmt.Errorf("failed to find verity (%s) data partition:\n%w", verityConfig.Id, err)
		}
		hashPartition, err := verityIdToPartition(verityConfig.HashDeviceId, verityConfig.HashDevice, partIdToPartUuid,
			diskPartitions)
		if err != nil {
			return nil, fmt.Errorf("failed to find verity (%s) hash partition:\n%w", verityConfig.Id, err)
		}

		// Format hash partition.
		rootHash, err := verityFormat(loopback.DevicePath(), dataPartition.Path, hashPartition.Path,
			shrinkHashPartition, sectorSize)
		if err != nil {
			return nil, err
		}

		metadata := verityDeviceMetadata{
			name:                  verityConfig.Name,
			rootHash:              rootHash,
			dataPartUuid:          dataPartition.PartUuid,
			hashPartUuid:          hashPartition.PartUuid,
			dataDeviceMountIdType: verityConfig.DataDeviceMountIdType,
			hashDeviceMountIdType: verityConfig.HashDeviceMountIdType,
			corruptionOption:      verityConfig.CorruptionOption,
			hashSignaturePath:     verityConfig.HashSignaturePath,
		}
		verityMetadata = append(verityMetadata, metadata)
	}

	// Refresh disk partitions after running veritysetup so that the hash partition's UUID is correct.
	err = diskutils.RefreshPartitions(loopback.DevicePath())
	if err != nil {
		return nil, err
	}

	diskPartitions, err = diskutils.GetDiskPartitions(loopback.DevicePath())
	if err != nil {
		return nil, err
	}

	// Update kernel args.
	isUki := config.OS.Uki != nil
	err = updateKernelArgsForVerity(buildDir, diskPartitions, verityMetadata, isUki)
	if err != nil {
		return nil, err
	}

	err = loopback.CleanClose()
	if err != nil {
		return nil, err
	}

	return verityMetadata, nil
}

func verityFormat(diskDevicePath string, dataPartitionPath string, hashPartitionPath string, shrinkHashPartition bool,
	sectorSize uint64,
) (string, error) {
	// Write hash partition.
	verityOutput, _, err := shell.NewExecBuilder("veritysetup", "format", dataPartitionPath, hashPartitionPath).
		LogLevel(logrus.DebugLevel, logrus.DebugLevel).
		ErrorStderrLines(1).
		ExecuteCaptureOutput()
	if err != nil {
		return "", fmt.Errorf("failed to calculate root hash (%s):\n%w", dataPartitionPath, err)
	}

	// Extract root hash using regular expressions.
	rootHashRegex, err := regexp.Compile(`Root hash:\s+([0-9a-fA-F]+)`)
	if err != nil {
		return "", fmt.Errorf("failed to compile root hash regex: %w", err)
	}

	rootHashMatches := rootHashRegex.FindStringSubmatch(verityOutput)
	if len(rootHashMatches) <= 1 {
		return "", fmt.Errorf("failed to parse root hash from veritysetup output")
	}

	rootHash := rootHashMatches[1]

	err = diskutils.RefreshPartitions(diskDevicePath)
	if err != nil {
		return "", fmt.Errorf("failed to wait for disk (%s) to update:\n%w", diskDevicePath, err)
	}

	if shrinkHashPartition {
		// Calculate the size of the hash partition from it's superblock.
		// In newer `veritysetup` versions, `veritysetup format` returns the size in its output. But that feature
		// is too new for now.
		hashPartitionSizeInBytes, err := calculateHashFileSizeInBytes(hashPartitionPath)
		if err != nil {
			return "", fmt.Errorf("failed to calculate hash partition's (%s) size:\n%w", hashPartitionPath, err)
		}

		hashPartitionSizeInSectors := convertBytesToSectors(hashPartitionSizeInBytes, sectorSize)

		err = resizePartition(hashPartitionPath, diskDevicePath, hashPartitionSizeInSectors)
		if err != nil {
			return "", fmt.Errorf("failed to shrink hash partition (%s):\n%w", diskDevicePath, err)
		}

		// Verify everything is still valid.
		err = shell.NewExecBuilder("veritysetup", "verify", dataPartitionPath, hashPartitionPath, rootHash).
			LogLevel(logrus.DebugLevel, logrus.DebugLevel).
			Execute()
		if err != nil {
			return "", fmt.Errorf("failed to verify verity (%s):\n%w", dataPartitionPath, err)
		}
	}

	return rootHash, nil
}

func updateKernelArgsForVerity(buildDir string, diskPartitions []diskutils.PartitionInfo,
	verityMetadata []verityDeviceMetadata, isUki bool,
) error {
	systemBootPartition, err := findSystemBootPartition(diskPartitions)
	if err != nil {
		return err
	}

	bootPartition, err := findBootPartitionFromEsp(systemBootPartition, diskPartitions, buildDir)
	if err != nil {
		return err
	}

	bootPartitionTmpDir := filepath.Join(buildDir, tmpBootPartitionDirName)
	// Temporarily mount the partition.
	bootPartitionMount, err := safemount.NewMount(bootPartition.Path, bootPartitionTmpDir, bootPartition.FileSystemType, 0, "", true)
	if err != nil {
		return fmt.Errorf("failed to mount partition (%s):\n%w", bootPartition.Path, err)
	}
	defer bootPartitionMount.Close()

	grubCfgFullPath := filepath.Join(bootPartitionTmpDir, DefaultGrubCfgPath)
	if err != nil {
		return fmt.Errorf("failed to stat file (%s):\n%w", grubCfgFullPath, err)
	}

	if isUki {
		// UKI is enabled, update kernel cmdline args file instead of grub.cfg.
		err = updateUkiKernelArgsForVerity(verityMetadata, diskPartitions, buildDir, bootPartition.Uuid)
		if err != nil {
			return fmt.Errorf("failed to update kernel cmdline arguments for verity:\n%w", err)
		}
	} else {
		// UKI is not enabled, update grub.cfg as usual.
		err = updateGrubConfigForVerity(verityMetadata, grubCfgFullPath, diskPartitions, buildDir, bootPartition.Uuid)
		if err != nil {
			return fmt.Errorf("failed to update grub config for verity:\n%w", err)
		}
	}

	err = bootPartitionMount.CleanClose()
	if err != nil {
		return err
	}

	return nil
}

func warnOnLowFreeSpace(buildDir string, imageConnection *ImageConnection) {
	logger.Log.Debugf("Checking disk space")

	imageChroot := imageConnection.Chroot()

	// Check all of the customized OS's partitions.
	for _, mountPoint := range getNonSpecialChrootMountPoints(imageConnection.Chroot()) {
		fullPath := filepath.Join(imageChroot.RootDir(), mountPoint.GetTarget())
		warnOnPathLowFreeSpace(fullPath, mountPoint.GetTarget())
	}

	// Check the partition that contains the build directory.
	warnOnPathLowFreeSpace(buildDir, "host:"+buildDir)
}

func warnOnPathLowFreeSpace(path string, name string) {
	var stat unix.Statfs_t
	err := unix.Statfs(path, &stat)
	if err != nil {
		logger.Log.Warnf("Failed to read disk space usage (%s)", path)
		return
	}

	totalBytes := stat.Frsize * int64(stat.Blocks)
	freeBytes := stat.Frsize * int64(stat.Bfree)
	usedBytes := totalBytes - freeBytes
	percentUsed := float64(usedBytes) / float64(totalBytes)
	percentFree := 1 - percentUsed

	logger.Log.Debugf("Disk space %.f%% (%s) on (%s)", percentUsed*100,
		humanReadableDiskSizeRatio(usedBytes, totalBytes), name)

	if percentFree <= diskFreeWarnThresholdPercent && freeBytes <= diskFreeWarnThresholdBytes {
		logger.Log.Warnf("Low free disk space %.f%% (%s) on (%s)", percentFree*100,
			humanReadableDiskSize(freeBytes), name)
	}
}

func humanReadableDiskSize(size int64) string {
	unitSize, unitName := humanReadableUnitSizeAndName(size)
	return fmt.Sprintf("%.f %s", float64(size)/float64(unitSize), unitName)
}

func humanReadableDiskSizeRatio(size int64, total int64) string {
	unitSize, unitName := humanReadableUnitSizeAndName(total)
	return fmt.Sprintf("%.f/%.f %s", float64(size)/float64(unitSize), float64(total)/float64(unitSize), unitName)
}

func humanReadableUnitSizeAndName(size int64) (int64, string) {
	switch {
	case size >= diskutils.TiB:
		return diskutils.TiB, "TiB"

	case size >= diskutils.GiB:
		return diskutils.GiB, "GiB"

	case size >= diskutils.MiB:
		return diskutils.MiB, "MiB"

	case size >= diskutils.KiB:
		return diskutils.KiB, "KiB"

	default:
		return 1, "B"
	}
}

func CheckEnvironmentVars() error {
	// Some commands, like tdnf (and gpg), require the USER and HOME environment variables to make sense in the OS they
	// are running under. Since the image customization tool is pretty much always run under root/sudo, this will
	// generally always be the case since root is always a valid user. However, this might not be true if the user
	// decides to use `sudo -E` instead of just `sudo`. So, check for this to avoid the user running into confusing
	// tdnf errors.
	//
	// In an ideal world, the USER, HOME, and PATH environment variables should be overridden whenever an external
	// command is called under chroot. But such a change would be quite involved.
	const (
		rootHome = "/root"
		rootUser = "root"
	)

	envHome := os.Getenv("HOME")
	envUser := os.Getenv("USER")

	if envHome != rootHome || (envUser != "" && envUser != rootUser) {
		return fmt.Errorf("tool should be run as root (e.g. by using sudo):\n"+
			"HOME must be set to '%s' (is '%s') and USER must be set to '%s' or '' (is '%s')",
			rootHome, envHome, rootUser, envUser)
	}

	return nil
}

func validateSnapshotTimeInput(snapshotTime string, previewFeatures []imagecustomizerapi.PreviewFeature) error {
	if snapshotTime != "" && !slices.Contains(previewFeatures, imagecustomizerapi.PreviewFeaturePackageSnapshotTime) {
		return fmt.Errorf("please enable the '%s' preview feature to specify '--package-snapshot-time'",
			imagecustomizerapi.PreviewFeaturePackageSnapshotTime)
	}

	if err := imagecustomizerapi.PackageSnapshotTime(snapshotTime).IsValid(); err != nil {
		return fmt.Errorf("invalid command-line option '--package-snapshot-time': '%s'\n%w", snapshotTime, err)
	}

	return nil
}
