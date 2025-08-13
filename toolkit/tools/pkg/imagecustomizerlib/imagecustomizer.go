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
	"github.com/microsoft/azurelinux/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/osinfo"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/randomization"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safemount"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/vhdutils"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"golang.org/x/sys/unix"
)

var (
	// Validation errors
	ErrInvalidOutputFormat            = NewImageCustomizerError("Validation:InvalidOutputFormat", "invalid output image format")
	ErrCannotGenerateOutputFormat     = NewImageCustomizerError("Validation:CannotGenerateOutputFormat", "cannot generate output format from input format")
	ErrCannotCustomizePartitionsOnIso = NewImageCustomizerError("Validation:CannotCustomizePartitionsOnIso", "cannot customize partitions when input is ISO")
	ErrInvalidImageConfig             = NewImageCustomizerError("Validation:InvalidImageConfig", "invalid image config")
	ErrInvalidParameters              = NewImageCustomizerError("Validation:InvalidParameters", "invalid parameters")
	ErrInputImageFileRequired         = NewImageCustomizerError("Validation:InputImageFileRequired", "input image file must be specified")
	ErrInvalidInputImageFileArg       = NewImageCustomizerError("Validation:InvalidInputImageFileArg", "invalid command-line option '--image-file'")
	ErrInputImageFileNotFile          = NewImageCustomizerError("Validation:InputImageFileNotFile", "input image file is not a file")
	ErrInvalidInputImageFileConfig    = NewImageCustomizerError("Validation:InvalidInputImageFileConfig", "invalid config file property 'input.image.path'")
	ErrInvalidAdditionalFilesSource   = NewImageCustomizerError("Validation:InvalidAdditionalFilesSource", "invalid additionalFiles source file")
	ErrAdditionalFilesSourceNotFile   = NewImageCustomizerError("Validation:AdditionalFilesSourceNotFile", "additionalFiles source file is not a file")
	ErrInvalidPostCustomizationScript = NewImageCustomizerError("Validation:InvalidPostCustomizationScript", "invalid postCustomization script")
	ErrInvalidFinalizeScript          = NewImageCustomizerError("Validation:InvalidFinalizeScript", "invalid finalizeCustomization script")
	ErrScriptNotUnderConfigDir        = NewImageCustomizerError("Validation:ScriptNotUnderConfigDir", "script file is not under config directory")
	ErrScriptFileNotReadable          = NewImageCustomizerError("Validation:ScriptFileNotReadable", "couldn't read script file")
	ErrNoRpmSourcesSpecified          = NewImageCustomizerError("Validation:NoRpmSourcesSpecified", "have packages to install or update but no RPM sources were specified")
	ErrOutputImageFileRequired        = NewImageCustomizerError("Validation:OutputImageFileRequired", "output image file must be specified")
	ErrInvalidOutputImageFileArg      = NewImageCustomizerError("Validation:InvalidOutputImageFileArg", "invalid command-line option '--output-image-file'")
	ErrOutputImageFileIsDirectory     = NewImageCustomizerError("Validation:OutputImageFileIsDirectory", "output image file is a directory")
	ErrInvalidOutputImageFileConfig   = NewImageCustomizerError("Validation:InvalidOutputImageFileConfig", "invalid config file property 'output.image.path'")
	ErrOutputImageFormatRequired      = NewImageCustomizerError("Validation:OutputImageFormatRequired", "output image format must be specified")
	ErrInvalidUser                    = NewImageCustomizerError("Validation:InvalidUser", "invalid user")
	ErrInvalidSSHPublicKeyFile        = NewImageCustomizerError("Validation:InvalidSSHPublicKeyFile", "failed to find SSH public key file")
	ErrSSHPublicKeyNotFile            = NewImageCustomizerError("Validation:SSHPublicKeyNotFile", "SSH public key path is not a file")
	ErrVerityValidation               = NewImageCustomizerError("Validation:VerityValidation", "verity validation failed")
	ErrUnsupportedQemuImageFormat     = NewImageCustomizerError("Validation:UnsupportedQemuImageFormat", "unsupported qemu-img format")
	ErrToolNotRunAsRoot               = NewImageCustomizerError("Validation:ToolNotRunAsRoot", "tool should be run as root (e.g. by using sudo)")
	ErrPackageSnapshotPreviewRequired = NewImageCustomizerError("Validation:PackageSnapshotPreviewRequired", fmt.Sprintf("preview feature '%s' required to specify package snapshot time", imagecustomizerapi.PreviewFeaturePackageSnapshotTime))
	ErrVerityPreviewFeatureRequired   = NewImageCustomizerError("Validation:VerityPreviewFeatureRequired", fmt.Sprintf("preview feature '%s' required to customize verity enabled base image", imagecustomizerapi.PreviewFeatureReinitializeVerity))
	ErrInvalidPackageSnapshotTime     = NewImageCustomizerError("Validation:InvalidPackageSnapshotTime", "invalid command-line option '--package-snapshot-time'")

	// Generic customization errors
	ErrGetAbsoluteConfigPath = NewImageCustomizerError("Customizer:GetAbsoluteConfigPath", "failed to get absolute path of config file directory")
	ErrCustomizeRawImage     = NewImageCustomizerError("Customizer:CustomizeRawImage", "failed to customize raw image")

	// Image conversion errors
	ErrConvertInputImage       = NewImageCustomizerError("ImageConversion:ConvertInput", "failed to convert input image to a raw image")
	ErrConvertToOutputFormat   = NewImageCustomizerError("ImageConversion:ConvertToOutput", "failed to convert customized raw image to output format")
	ErrDetectImageFormat       = NewImageCustomizerError("ImageConversion:DetectFormat", "failed to detect input image format")
	ErrConvertImageToRawFormat = NewImageCustomizerError("ImageConversion:ConvertToRawFormat", "failed to convert image file to raw format")
	ErrConvertImageToFormat    = NewImageCustomizerError("ImageConversion:ConvertToFormat", "failed to convert image file to format")

	// Artifacts errors
	ErrExtractPackages           = NewImageCustomizerError("Artifacts:ExtractPackages", "failed to extract installed packages")
	ErrExtractBootloaderMetadata = NewImageCustomizerError("Artifacts:ExtractBootloaderMetadata", "failed to extract bootloader metadata")
	ErrCollectOSInfo             = NewImageCustomizerError("Artifacts:CollectOSInfo", "failed to collect OS information")

	// LiveOS errors
	ErrCreateArtifactsStore  = NewImageCustomizerError("LiveOS:CreateArtifactsStore", "failed to create artifacts store")
	ErrBuildLiveOSConfig     = NewImageCustomizerError("LiveOS:BuildConfig", "failed to build Live OS configuration")
	ErrCreateWriteableImage  = NewImageCustomizerError("LiveOS:CreateWriteableImage", "failed to create writeable image")
	ErrCreateLiveOSArtifacts = NewImageCustomizerError("LiveOS:CreateArtifacts", "failed to create Live OS artifacts")

	// Filesystem errors
	ErrShrinkFilesystems = NewImageCustomizerError("Filesystem:Shrink", "failed to shrink filesystems")
	ErrCheckFilesystems  = NewImageCustomizerError("Filesystem:Check", "failed to check filesystems")
	ErrStatFile          = NewImageCustomizerError("Filesystem:StatFile", "failed to stat file")

	// Verity errors
	ErrVerityImageConnection   = NewImageCustomizerError("Verity:ConnectToImage", "failed to connect to image file to provision verity")
	ErrGetDiskSectorSize       = NewImageCustomizerError("Verity:GetSectorSize", "failed to get disk sector size")
	ErrMountPartition          = NewImageCustomizerError("Verity:MountPartition", "failed to mount partition")
	ErrUpdateDisk              = NewImageCustomizerError("Verity:UpdateDisk", "failed to wait for disk to update")
	ErrFindVerityDataPartition = NewImageCustomizerError("Verity:FindDataPartition", "failed to find verity data partition")
	ErrFindVerityHashPartition = NewImageCustomizerError("Verity:FindHashPartition", "failed to find verity hash partition")
	ErrCalculateRootHash       = NewImageCustomizerError("Verity:CalculateRootHash", "failed to calculate root hash")
	ErrCompileRootHashRegex    = NewImageCustomizerError("Verity:CompileRootHashRegex", "failed to compile root hash regex")
	ErrParseRootHash           = NewImageCustomizerError("Verity:ParseRootHash", "failed to parse root hash from veritysetup output")
	ErrCalculateHashSize       = NewImageCustomizerError("Verity:CalculateHashSize", "failed to calculate hash partition size")
	ErrShrinkHashPartition     = NewImageCustomizerError("Verity:ShrinkHashPartition", "failed to shrink hash partition")
	ErrVerifyVerity            = NewImageCustomizerError("Verity:Verify", "failed to verify verity")
	ErrUpdateKernelArgs        = NewImageCustomizerError("Verity:UpdateKernelArgs", "failed to update kernel cmdline arguments for verity")
	ErrUpdateGrubConfig        = NewImageCustomizerError("Verity:UpdateGrubConfig", "failed to update grub config for verity")
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
	cosiBootMetadata     *CosiBootloader
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

func createImageCustomizerParameters(ctx context.Context, buildDir string,
	inputImageFile string,
	configPath string, config *imagecustomizerapi.Config,
	useBaseImageRpmRepos bool, rpmsSources []string,
	outputImageFormat string, outputImageFile string, packageSnapshotTime string,
) (*ImageCustomizerParameters, error) {
	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "create_image_customizer_parameters")
	defer span.End()

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
		return nil, fmt.Errorf("%w (format='%s'):\n%w", ErrInvalidOutputFormat, outputImageFormat, err)
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
			return nil, fmt.Errorf("%w (output='%s', input='%s')", ErrCannotGenerateOutputFormat, ic.outputImageFormat, ic.inputImageFormat)
		}

		// While defining a storage configuration can work when the input image is
		// an iso, there is no obvious point of moving content between partitions
		// where all partitions get collapsed into the squashfs at the end.
		if config.CustomizePartitions() {
			return nil, ErrCannotCustomizePartitionsOnIso
		}
	}

	ic.packageSnapshotTime = packageSnapshotTime

	return ic, nil
}

func CustomizeImageWithConfigFile(ctx context.Context, buildDir string, configFile string, inputImageFile string,
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
		return fmt.Errorf("%w:\n%w", ErrGetAbsoluteConfigPath, err)
	}

	err = CustomizeImage(ctx, buildDir, absBaseConfigPath, &config, inputImageFile, rpmsSources, outputImageFile, outputImageFormat,
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

func CustomizeImage(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.Config, inputImageFile string,
	rpmsSources []string, outputImageFile string, outputImageFormat string,
	useBaseImageRpmRepos bool, packageSnapshotTime string,
) (err error) {
	ctx, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "customize_image")
	span.SetAttributes(
		attribute.String("output_image_format", string(outputImageFormat)),
	)
	defer func() {
		if err != nil {
			errorNames := []string{"Unset"} // default
			if namedErrors := GetAllImageCustomizerErrors(err); len(namedErrors) > 0 {
				errorNames = make([]string, len(namedErrors))
				for i, namedError := range namedErrors {
					errorNames[i] = namedError.Name()
				}
			}
			span.SetAttributes(
				attribute.StringSlice("errors.name", errorNames),
			)
			span.SetStatus(codes.Error, errorNames[len(errorNames)-1])
		}
		span.End()
	}()

	err = ValidateConfig(ctx, baseConfigPath, config, inputImageFile, rpmsSources, outputImageFile, outputImageFormat, useBaseImageRpmRepos, packageSnapshotTime, false)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrInvalidImageConfig, err)
	}

	imageCustomizerParameters, err := createImageCustomizerParameters(ctx, buildDir, inputImageFile,
		baseConfigPath, config, useBaseImageRpmRepos, rpmsSources,
		outputImageFormat, outputImageFile, packageSnapshotTime)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrInvalidParameters, err)
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

	inputIsoArtifacts, err := convertInputImageToWriteableFormat(ctx, imageCustomizerParameters)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrConvertInputImage, err)
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

	err = customizeOSContents(ctx, imageCustomizerParameters)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrCustomizeRawImage, err)
	}

	if config.Output.Artifacts != nil {
		outputDir := file.GetAbsPathWithBase(baseConfigPath, config.Output.Artifacts.Path)

		err = outputArtifacts(ctx, config.Output.Artifacts.Items, outputDir, imageCustomizerParameters.buildDirAbs,
			imageCustomizerParameters.rawImageFile, imageCustomizerParameters.verityMetadata)
		if err != nil {
			return err
		}
	}

	err = convertWriteableFormatToOutputImage(ctx, imageCustomizerParameters, inputIsoArtifacts)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrConvertToOutputFormat, err)
	}

	logger.Log.Infof("Success!")

	return nil
}

func isKdumpBootFilesConfigChanging(requestedKdumpBootFiles *imagecustomizerapi.KdumpBootFilesType,
	inputKdumpBootFiles *imagecustomizerapi.KdumpBootFilesType) bool {
	// If the requested kdump boot files is nil, it means that the user did not
	// specify a kdump boot files configuration, so it is definitely not changing
	// when compared to the previous run.
	if requestedKdumpBootFiles == nil {
		return false
	}

	requestedKdumpBootFilesCfg := *requestedKdumpBootFiles == imagecustomizerapi.KdumpBootFilesTypeKeep

	// The default value for inputKdumpBootFilesCfg is false because in case of the absence of
	// inputKdumpBootFiles, the implied configuration is false (KdumpBootFilesTypeKeepNone).
	inputKdumpBootFilesCfg := false
	if inputKdumpBootFiles != nil {
		inputKdumpBootFilesCfg = *inputKdumpBootFiles == imagecustomizerapi.KdumpBootFilesTypeKeep
	}

	return requestedKdumpBootFilesCfg != inputKdumpBootFilesCfg
}

func convertInputImageToWriteableFormat(ctx context.Context, ic *ImageCustomizerParameters) (*IsoArtifactsStore, error) {
	logger.Log.Infof("Converting input image to a writeable format")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "input_image_conversion")
	span.SetAttributes(
		attribute.String("input_image_format", ic.inputImageFormat),
	)
	defer span.End()

	if ic.inputIsIso {

		inputIsoArtifacts, err := createIsoArtifactStoreFromIsoImage(ic.inputImageFile, filepath.Join(ic.buildDirAbs, "from-iso"))
		if err != nil {
			return inputIsoArtifacts, fmt.Errorf("%w (source='%s'):\n%w", ErrCreateArtifactsStore, ic.inputImageFile, err)
		}

		var liveosConfig LiveOSConfig
		liveosConfig, convertInitramfsType, err := buildLiveOSConfig(inputIsoArtifacts, ic.config.Iso,
			ic.config.Pxe, ic.outputImageFormat)
		if err != nil {
			return nil, fmt.Errorf("%w:\n%w", ErrBuildLiveOSConfig, err)
		}

		// Check if the user is changing the kdump boot files configuration.
		// If it is changing, it may change the composition of the full OS
		// image, and a reconstruction of the full OS image is needed.
		kdumpBootFileChanging := isKdumpBootFilesConfigChanging(liveosConfig.kdumpBootFiles, inputIsoArtifacts.info.kdumpBootFiles)

		// If the input is a LiveOS iso and there are OS customizations
		// defined, we create a writeable disk image so that mic can modify
		// it. If no OS customizations are defined, we can skip this step and
		// just re-use the existing squashfs.
		rebuildFullOsImage := ic.customizeOSPartitions || convertInitramfsType || kdumpBootFileChanging

		if rebuildFullOsImage {
			err = createWriteableImageFromArtifacts(ic.buildDirAbs, inputIsoArtifacts, ic.rawImageFile)
			if err != nil {
				return nil, fmt.Errorf("%w:\n%w", ErrCreateWriteableImage, err)
			}
		}

		return inputIsoArtifacts, nil
	} else {
		logger.Log.Infof("Creating raw base image: %s", ic.rawImageFile)

		_, err := convertImageToRaw(ic.inputImageFile, ic.rawImageFile)
		if err != nil {
			return nil, err
		}

		return nil, nil
	}
}

func convertImageToRaw(inputImageFile string, rawImageFile string) (imagecustomizerapi.ImageFormatType, error) {
	imageInfo, err := GetImageFileInfo(inputImageFile)
	if err != nil {
		return "", fmt.Errorf("%w (file='%s'):\n%w", ErrDetectImageFormat, inputImageFile, err)
	}

	detectedImageFormat := imageInfo.Format
	sourceArg := fmt.Sprintf("file.filename=%s", qemuImgEscapeOptionValue(inputImageFile))

	if detectedImageFormat == "raw" || detectedImageFormat == "vpc" {
		// The fixed-size VHD format is just a raw disk file with small metadata footer appended to the end.
		// Unfortunatley, qemu-img doesn't look at the VHD footer when detecting file formats. So, it reports
		// fixed-sized VHDs as raw disk images. So, manually detect if a raw image is a VHD.
		vhdFileType, err := vhdutils.GetVhdFileSizeCalcType(inputImageFile)
		if err != nil {
			return "", err
		}

		switch vhdFileType {
		case vhdutils.VhdFileSizeCalcTypeDiskGeometry:
			return "", fmt.Errorf("rejecting VHD file that uses 'Disk Geometry' based size:\npass '-o force_size=on' to qemu-img when outputting as 'vpc' (i.e. VHD)")

		case vhdutils.VhdFileSizeCalcTypeCurrentSize:
			sourceArg += ",driver=vpc,force_size_calc=current_size"
			detectedImageFormat = "vpc"

		default:
			// Not a VHD file.
		}
	}

	err = shell.ExecuteLiveWithErr(1, "qemu-img", "convert", "-O", "raw", "--image-opts", sourceArg, rawImageFile)
	if err != nil {
		return "", fmt.Errorf("%w:\n%w", ErrConvertImageToRawFormat, err)
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
		return "", fmt.Errorf("%w: %s", ErrUnsupportedQemuImageFormat, qemuFormat)
	}
}

func qemuImgEscapeOptionValue(value string) string {
	// Commas are escaped by doubling them up.
	return strings.ReplaceAll(value, ",", ",,")
}

func customizeOSContents(ctx context.Context, ic *ImageCustomizerParameters) error {
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

	ctx, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "customize_os_contents")
	defer span.End()

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
	partitionsCustomized, newRawImageFile, partIdToPartUuid, err := customizePartitions(ctx, ic.buildDirAbs,
		ic.configPath, ic.config, ic.rawImageFile)
	if err != nil {
		return err
	}

	if ic.rawImageFile != newRawImageFile {
		os.Remove(ic.rawImageFile)
		ic.rawImageFile = newRawImageFile
	}

	// Customize the raw image file.
	partUuidToFstabEntry, baseImageVerityMetadata, readonlyPartUuids, osRelease, err := customizeImageHelper(ctx,
		ic.buildDirAbs, ic.configPath, ic.config, ic.rawImageFile, ic.rpmsSources, ic.useBaseImageRpmRepos,
		partitionsCustomized, ic.imageUuidStr, ic.packageSnapshotTime, ic.outputImageFormat)
	if err != nil {
		return err
	}

	if len(baseImageVerityMetadata) > 0 {
		previewFeatureEnabled := slices.Contains(ic.config.PreviewFeatures,
			imagecustomizerapi.PreviewFeatureReinitializeVerity)
		if !previewFeatureEnabled {
			return ErrVerityPreviewFeatureRequired
		}
	}

	ic.partUuidToFstabEntry = partUuidToFstabEntry
	ic.baseImageVerityMetadata = baseImageVerityMetadata
	ic.osRelease = osRelease

	// For COSI, always shrink the filesystems.
	shrinkPartitions := ic.outputImageFormat == imagecustomizerapi.ImageFormatTypeCosi
	if shrinkPartitions {
		err = shrinkFilesystemsHelper(ctx, ic.rawImageFile, readonlyPartUuids)
		if err != nil {
			return fmt.Errorf("%w:\n%w", ErrShrinkFilesystems, err)
		}
	}

	if len(ic.config.Storage.Verity) > 0 || len(ic.baseImageVerityMetadata) > 0 {
		// Customize image for dm-verity, setting up verity metadata and security features.
		verityMetadata, err := customizeVerityImageHelper(ctx, ic.buildDirAbs, ic.config, ic.rawImageFile, partIdToPartUuid,
			shrinkPartitions, ic.baseImageVerityMetadata, readonlyPartUuids)
		if err != nil {
			return err
		}

		ic.verityMetadata = verityMetadata
	}

	if ic.config.OS.Uki != nil {
		err = createUki(ctx, ic.config.OS.Uki, ic.buildDirAbs, ic.rawImageFile)
		if err != nil {
			return err
		}
	}

	// collect OS info if generating a COSI image
	var osPackages []OsPackage
	var cosiBootMetadata *CosiBootloader
	if ic.config.Output.Image.Format == imagecustomizerapi.ImageFormatTypeCosi || ic.outputImageFormat == imagecustomizerapi.ImageFormatTypeCosi {
		osPackages, cosiBootMetadata, err = collectOSInfo(ctx, ic.buildDirAbs, ic.rawImageFile)
		if err != nil {
			return fmt.Errorf("%w:\n%w", ErrCollectOSInfo, err)
		}
		ic.osPackages = osPackages
		ic.cosiBootMetadata = cosiBootMetadata
	}

	// Check file systems for corruption.
	err = checkFileSystems(ctx, ic.rawImageFile)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrCheckFilesystems, err)
	}

	return nil
}

func convertWriteableFormatToOutputImage(ctx context.Context, ic *ImageCustomizerParameters, inputIsoArtifacts *IsoArtifactsStore) error {
	logger.Log.Infof("Converting customized OS partitions into the final image")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "output_image_conversion")
	span.SetAttributes(
		attribute.String("output_image_format", string(ic.outputImageFormat)),
	)
	defer span.End()

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
			ic.verityMetadata, ic.osRelease, ic.osPackages, ic.imageUuid, ic.imageUuidStr, ic.cosiBootMetadata)
		if err != nil {
			return err
		}

	case imagecustomizerapi.ImageFormatTypeIso, imagecustomizerapi.ImageFormatTypePxeDir, imagecustomizerapi.ImageFormatTypePxeTar:
		// Decide whether we need to re-build the full OS image or not
		convertInitramfsType := false
		kdumpBootFileChanging := false
		if inputIsoArtifacts != nil {
			// Check if user is converting from full os initramfs to bootstrap initramfs
			var err error
			var liveosConfig LiveOSConfig
			liveosConfig, convertInitramfsType, err = buildLiveOSConfig(inputIsoArtifacts, ic.config.Iso, ic.config.Pxe,
				ic.outputImageFormat)
			if err != nil {
				return fmt.Errorf("%w:\n%w", ErrBuildLiveOSConfig, err)
			}

			// Check if the user is changing the kdump boot files configuration.
			// If it is changing, it may change the composition of the full OS
			// image, and a reconstruction of the full OS image is needed.
			kdumpBootFileChanging = isKdumpBootFilesConfigChanging(liveosConfig.kdumpBootFiles, inputIsoArtifacts.info.kdumpBootFiles)
		}

		rebuildFullOsImage := ic.customizeOSPartitions || inputIsoArtifacts == nil || convertInitramfsType || kdumpBootFileChanging

		// Either re-build the full OS image, or just re-package the existing one
		if rebuildFullOsImage {
			requestedSELinuxMode := imagecustomizerapi.SELinuxModeDefault
			if ic.config.OS != nil {
				requestedSELinuxMode = ic.config.OS.SELinux.Mode
			}
			err := createLiveOSFromRaw(ctx, ic.buildDirAbs, ic.configPath, inputIsoArtifacts, requestedSELinuxMode,
				ic.config.Iso, ic.config.Pxe, ic.rawImageFile, ic.outputImageFormat, ic.outputImageFile)
			if err != nil {
				return fmt.Errorf("%w:\n%w", ErrCreateLiveOSArtifacts, err)
			}
		} else {
			err := repackageLiveOS(ic.buildDirAbs, ic.configPath, ic.config.Iso, ic.config.Pxe,
				inputIsoArtifacts, ic.outputImageFormat, ic.outputImageFile)
			if err != nil {
				return fmt.Errorf("%w:\n%w", ErrCreateLiveOSArtifacts, err)
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
		return fmt.Errorf("%w (format='%s'):\n%w", ErrConvertImageToFormat, format, err)
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

func ValidateConfig(ctx context.Context, baseConfigPath string, config *imagecustomizerapi.Config, inputImageFile string, rpmsSources []string,
	outputImageFile, outputImageFormat string, useBaseImageRpmRepos bool, packageSnapshotTime string, newImage bool,
) error {
	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "validate_config")
	defer span.End()

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
		return ErrInputImageFileRequired
	}

	if inputImageFile != "" {
		if yes, err := file.IsFile(inputImageFile); err != nil {
			return fmt.Errorf("%w (file='%s'):\n%w", ErrInvalidInputImageFileArg, inputImageFile, err)
		} else if !yes {
			return fmt.Errorf("%w (file='%s')", ErrInputImageFileNotFile, inputImageFile)
		}
	} else {
		inputImageAbsPath := file.GetAbsPathWithBase(baseConfigPath, input.Image.Path)
		if yes, err := file.IsFile(inputImageAbsPath); err != nil {
			return fmt.Errorf("%w (path='%s'):\n%w", ErrInvalidInputImageFileConfig, input.Image.Path, err)
		} else if !yes {
			return fmt.Errorf("%w (path='%s')", ErrInputImageFileNotFile, input.Image.Path)
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
				errs = append(errs, fmt.Errorf("%w (source='%s'):\n%w", ErrInvalidAdditionalFilesSource, additionalFile.Source, err))
			}

			if !isFile {
				errs = append(errs, fmt.Errorf("%w (source='%s')", ErrAdditionalFilesSourceNotFile,
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
			return fmt.Errorf("%w (index=%d):\n%w", ErrInvalidPostCustomizationScript, i, err)
		}
	}

	for i, script := range scripts.FinalizeCustomization {
		err := validateScript(baseConfigPath, &script)
		if err != nil {
			return fmt.Errorf("%w (index=%d):\n%w", ErrInvalidFinalizeScript, i, err)
		}
	}

	return nil
}

func validateScript(baseConfigPath string, script *imagecustomizerapi.Script) error {
	if script.Path != "" {
		// Ensure that install scripts sit under the config file's parent directory.
		// This allows the install script to be run in the chroot environment by bind mounting the config directory.
		if !filepath.IsLocal(script.Path) {
			return fmt.Errorf("%w (script='%s', config='%s')", ErrScriptNotUnderConfigDir, script.Path, baseConfigPath)
		}

		fullPath := filepath.Join(baseConfigPath, script.Path)

		// Verify that the file exists.
		_, err := os.Stat(fullPath)
		if err != nil {
			return fmt.Errorf("%w (script='%s'):\n%w", ErrScriptFileNotReadable, script.Path, err)
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
			return ErrNoRpmSourcesSpecified
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
		return ErrOutputImageFileRequired
	}

	// Pxe output format allows the output to be a path.
	if output.Image.Format != imagecustomizerapi.ImageFormatTypePxeDir && outputImageFormat != string(imagecustomizerapi.ImageFormatTypePxeDir) {
		if outputImageFile != "" {
			if isDir, err := file.DirExists(outputImageFile); err != nil {
				return fmt.Errorf("%w (file='%s'):\n%w", ErrInvalidOutputImageFileArg, outputImageFile, err)
			} else if isDir {
				return fmt.Errorf("%w (file='%s')", ErrOutputImageFileIsDirectory, outputImageFile)
			}
		} else {
			outputImageAbsPath := file.GetAbsPathWithBase(baseConfigPath, output.Image.Path)
			if isDir, err := file.DirExists(outputImageAbsPath); err != nil {
				return fmt.Errorf("%w (path='%s'):\n%w", ErrInvalidOutputImageFileConfig, output.Image.Path, err)
			} else if isDir {
				return fmt.Errorf("%w (path='%s')", ErrOutputImageFileIsDirectory, output.Image.Path)
			}
		}
	}

	if outputImageFormat == "" && output.Image.Format == imagecustomizerapi.ImageFormatTypeNone {
		return ErrOutputImageFormatRequired
	}

	return nil
}

func validateUsers(baseConfigPath string, users []imagecustomizerapi.User) error {
	for _, user := range users {
		err := validateUser(baseConfigPath, user)
		if err != nil {
			return fmt.Errorf("%w (user='%s'):\n%w", ErrInvalidUser, user.Name, err)
		}
	}

	return nil
}

func validateUser(baseConfigPath string, user imagecustomizerapi.User) error {
	for _, path := range user.SSHPublicKeyPaths {
		absPath := file.GetAbsPathWithBase(baseConfigPath, path)
		isFile, err := file.IsFile(absPath)
		if err != nil {
			return fmt.Errorf("%w (path='%s'):\n%w", ErrInvalidSSHPublicKeyFile, path, err)
		}
		if !isFile {
			return fmt.Errorf("%w (path='%s')", ErrSSHPublicKeyNotFile, path)
		}
	}

	return nil
}

func customizeImageHelper(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.Config,
	rawImageFile string, rpmsSources []string, useBaseImageRpmRepos bool, partitionsCustomized bool,
	imageUuidStr string, packageSnapshotTime string, outputImageFormatType imagecustomizerapi.ImageFormatType,
) (map[string]diskutils.FstabEntry, []verityDeviceMetadata, []string, string, error) {
	logger.Log.Debugf("Customizing OS")

	readOnlyVerity := config.Storage.ReinitializeVerity != imagecustomizerapi.ReinitializeVerityTypeAll

	imageConnection, partUuidToFstabEntry, baseImageVerityMetadata, readonlyPartUuids, err := connectToExistingImage(
		ctx, rawImageFile, buildDir, "imageroot", true, false, readOnlyVerity)
	if err != nil {
		return nil, nil, nil, "", err
	}
	defer imageConnection.Close()

	osRelease, err := extractOSRelease(imageConnection)
	if err != nil {
		return nil, nil, nil, "", err
	}

	imageConnection.Chroot().UnsafeRun(func() error {
		distro, version := osinfo.GetDistroAndVersion()
		logger.Log.Infof("Base OS distro: %s", distro)
		logger.Log.Infof("Base OS version: %s", version)
		return nil
	})

	err = validateVerityMountPaths(imageConnection, config, partUuidToFstabEntry, baseImageVerityMetadata)
	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("%w:\n%w", ErrVerityValidation, err)
	}

	// Do the actual customizations.
	err = doOsCustomizations(ctx, buildDir, baseConfigPath, config, imageConnection, rpmsSources,
		useBaseImageRpmRepos, partitionsCustomized, imageUuidStr, partUuidToFstabEntry, packageSnapshotTime)

	// Out of disk space errors can be difficult to diagnose.
	// So, warn about any partitions with low free space.
	warnOnLowFreeSpace(buildDir, imageConnection)

	if err != nil {
		return nil, nil, nil, "", err
	}

	err = imageConnection.CleanClose()
	if err != nil {
		return nil, nil, nil, "", err
	}

	return partUuidToFstabEntry, baseImageVerityMetadata, readonlyPartUuids, osRelease, nil
}

func collectOSInfo(ctx context.Context, buildDir string, rawImageFile string,
) ([]OsPackage, *CosiBootloader, error) {
	var err error
	imageConnection, _, _, _, err := connectToExistingImage(ctx, rawImageFile, buildDir, "imageroot", true, true, false)
	if err != nil {
		return nil, nil, err
	}
	defer imageConnection.Close()

	osPackages, cosiBootMetadata, err := collectOSInfoHelper(ctx, buildDir, imageConnection)
	if err != nil {
		return nil, nil, err
	}

	err = imageConnection.CleanClose()
	if err != nil {
		return nil, nil, err
	}

	return osPackages, cosiBootMetadata, nil
}

func collectOSInfoHelper(ctx context.Context, buildDir string, imageConnection *imageconnection.ImageConnection) ([]OsPackage, *CosiBootloader, error) {
	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "collect_os_info")
	defer span.End()
	osPackages, err := getAllPackagesFromChroot(imageConnection)
	if err != nil {
		return nil, nil, fmt.Errorf("%w:\n%w", ErrExtractPackages, err)
	}

	cosiBootMetadata, err := extractCosiBootMetadata(buildDir, imageConnection)
	if err != nil {
		return nil, nil, fmt.Errorf("%w:\n%w", ErrExtractBootloaderMetadata, err)
	}

	return osPackages, cosiBootMetadata, nil
}

func shrinkFilesystemsHelper(ctx context.Context, buildImageFile string, readonlyPartUuids []string) error {
	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "shrink_filesystems")
	defer span.End()

	imageLoopback, err := safeloopback.NewLoopback(buildImageFile)
	if err != nil {
		return err
	}
	defer imageLoopback.Close()

	// Shrink the filesystems.
	err = shrinkFilesystems(imageLoopback.DevicePath(), readonlyPartUuids)
	if err != nil {
		return err
	}

	err = imageLoopback.CleanClose()
	if err != nil {
		return err
	}

	return nil
}

func customizeVerityImageHelper(ctx context.Context, buildDir string, config *imagecustomizerapi.Config,
	buildImageFile string, partIdToPartUuid map[string]string, shrinkHashPartition bool,
	baseImageVerity []verityDeviceMetadata, readonlyPartUuids []string,
) ([]verityDeviceMetadata, error) {
	logger.Log.Infof("Provisioning verity")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "provision_verity")
	defer span.End()

	verityMetadata := []verityDeviceMetadata(nil)

	loopback, err := safeloopback.NewLoopback(buildImageFile)
	if err != nil {
		return nil, fmt.Errorf("%w:\n%w", ErrVerityImageConnection, err)
	}
	defer loopback.Close()

	diskPartitions, err := diskutils.GetDiskPartitions(loopback.DevicePath())
	if err != nil {
		return nil, err
	}

	sectorSize, _, err := diskutils.GetSectorSize(loopback.DevicePath())
	if err != nil {
		return nil, fmt.Errorf("%w (device='%s'):\n%w", ErrGetDiskSectorSize, loopback.DevicePath(), err)
	}

	verityUpdated := false

	for _, metadata := range baseImageVerity {
		newMetadata := metadata

		readonly := slices.Contains(readonlyPartUuids, metadata.dataPartUuid)
		if !readonly {
			// Find partitions.
			dataPartition, _, err := findPartitionHelper(imagecustomizerapi.MountIdentifierTypePartUuid,
				metadata.dataPartUuid, diskPartitions)
			if err != nil {
				return nil, fmt.Errorf("%w (name='%s'):\n%w", ErrFindVerityDataPartition, metadata.name, err)
			}

			hashPartition, _, err := findPartitionHelper(imagecustomizerapi.MountIdentifierTypePartUuid,
				metadata.hashPartUuid, diskPartitions)
			if err != nil {
				return nil, fmt.Errorf("%w (name='%s'):\n%w", ErrFindVerityHashPartition, metadata.name, err)
			}

			// Format hash partition.
			rootHash, err := verityFormat(loopback.DevicePath(), dataPartition.Path, hashPartition.Path,
				shrinkHashPartition, sectorSize)
			if err != nil {
				return nil, err
			}

			newMetadata.rootHash = rootHash
			verityUpdated = true
		}

		verityMetadata = append(verityMetadata, newMetadata)
	}

	for _, verityConfig := range config.Storage.Verity {
		// Extract the partition block device path.
		dataPartition, err := verityIdToPartition(verityConfig.DataDeviceId, verityConfig.DataDevice, partIdToPartUuid,
			diskPartitions)
		if err != nil {
			return nil, fmt.Errorf("%w (id='%s'):\n%w", ErrFindVerityDataPartition, verityConfig.Id, err)
		}
		hashPartition, err := verityIdToPartition(verityConfig.HashDeviceId, verityConfig.HashDevice, partIdToPartUuid,
			diskPartitions)
		if err != nil {
			return nil, fmt.Errorf("%w (id='%s'):\n%w", ErrFindVerityHashPartition, verityConfig.Id, err)
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
		verityUpdated = true
	}

	// Refresh disk partitions after running veritysetup so that the hash partition's UUID is correct.
	err = diskutils.RefreshPartitions(loopback.DevicePath())
	if err != nil {
		return nil, err
	}

	if verityUpdated {
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
	}

	err = loopback.CleanClose()
	if err != nil {
		return nil, err
	}

	deviceNamesJson := getVerityNames(verityMetadata)
	span.SetAttributes(
		attribute.Int("verity_count", len(verityMetadata)),
		attribute.StringSlice("verity_device_name", deviceNamesJson),
	)

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
		return "", fmt.Errorf("%w (partition='%s'):\n%w", ErrCalculateRootHash, dataPartitionPath, err)
	}

	// Extract root hash using regular expressions.
	rootHashRegex, err := regexp.Compile(`Root hash:\s+([0-9a-fA-F]+)`)
	if err != nil {
		return "", fmt.Errorf("%w:\n%w", ErrCompileRootHashRegex, err)
	}

	rootHashMatches := rootHashRegex.FindStringSubmatch(verityOutput)
	if len(rootHashMatches) <= 1 {
		return "", ErrParseRootHash
	}

	rootHash := rootHashMatches[1]

	err = diskutils.RefreshPartitions(diskDevicePath)
	if err != nil {
		return "", fmt.Errorf("%w (device='%s'):\n%w", ErrUpdateDisk, diskDevicePath, err)
	}

	if shrinkHashPartition {
		// Calculate the size of the hash partition from it's superblock.
		// In newer `veritysetup` versions, `veritysetup format` returns the size in its output. But that feature
		// is too new for now.
		hashPartitionSizeInBytes, err := calculateHashFileSizeInBytes(hashPartitionPath)
		if err != nil {
			return "", fmt.Errorf("%w (partition='%s'):\n%w", ErrCalculateHashSize, hashPartitionPath, err)
		}

		hashPartitionSizeInSectors := convertBytesToSectors(hashPartitionSizeInBytes, sectorSize)

		err = resizePartition(hashPartitionPath, diskDevicePath, hashPartitionSizeInSectors)
		if err != nil {
			return "", fmt.Errorf("%w (device='%s'):\n%w", ErrShrinkHashPartition, diskDevicePath, err)
		}

		// Verify everything is still valid.
		err = shell.NewExecBuilder("veritysetup", "verify", dataPartitionPath, hashPartitionPath, rootHash).
			LogLevel(logrus.DebugLevel, logrus.DebugLevel).
			Execute()
		if err != nil {
			return "", fmt.Errorf("%w (partition='%s'):\n%w", ErrVerifyVerity, dataPartitionPath, err)
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
		return fmt.Errorf("%w (partition='%s'):\n%w", ErrMountPartition, bootPartition.Path, err)
	}
	defer bootPartitionMount.Close()

	grubCfgFullPath := filepath.Join(bootPartitionTmpDir, DefaultGrubCfgPath)
	_, err = os.Stat(grubCfgFullPath)
	if err != nil {
		return fmt.Errorf("%w (file='%s'):\n%w", ErrStatFile, grubCfgFullPath, err)
	}

	if isUki {
		// UKI is enabled, update kernel cmdline args file.
		err = updateUkiKernelArgsForVerity(verityMetadata, diskPartitions, buildDir, bootPartition.Uuid)
		if err != nil {
			return fmt.Errorf("%w:\n%w", ErrUpdateKernelArgs, err)
		}
	}

	// Temporarily always update grub.cfg for verity, even when UKI is used.
	// Since grub dependencies are still kept under /boot and won't be cleaned.
	// This will be decoupled once the bootloader project is in place.
	err = updateGrubConfigForVerity(verityMetadata, grubCfgFullPath, diskPartitions, buildDir, bootPartition.Uuid)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrUpdateGrubConfig, err)
	}

	err = bootPartitionMount.CleanClose()
	if err != nil {
		return err
	}

	return nil
}

func warnOnLowFreeSpace(buildDir string, imageConnection *imageconnection.ImageConnection) {
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
		return ErrToolNotRunAsRoot
	}

	return nil
}

func validateSnapshotTimeInput(snapshotTime string, previewFeatures []imagecustomizerapi.PreviewFeature) error {
	if snapshotTime != "" && !slices.Contains(previewFeatures, imagecustomizerapi.PreviewFeaturePackageSnapshotTime) {
		return ErrPackageSnapshotPreviewRequired
	}

	if err := imagecustomizerapi.PackageSnapshotTime(snapshotTime).IsValid(); err != nil {
		return fmt.Errorf("%w (time='%s'):\n%w", ErrInvalidPackageSnapshotTime, snapshotTime, err)
	}

	return nil
}
