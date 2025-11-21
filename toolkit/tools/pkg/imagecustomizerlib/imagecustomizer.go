// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/osinfo"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/vhdutils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"golang.org/x/sys/unix"
)

var (
	// Validation errors
	ErrInvalidOutputFormat            = NewImageCustomizerError("Validation:InvalidOutputFormat", "invalid output image format")
	ErrCannotGenerateOutputFormat     = NewImageCustomizerError("Validation:CannotGenerateOutputFormat", "cannot generate output format from input format")
	ErrCannotValidateTargetOS         = NewImageCustomizerError("Validation:CannotValidateTargetOS", "cannot validate target OS of the base image")
	ErrCannotCustomizePartitionsOnIso = NewImageCustomizerError("Validation:CannotCustomizePartitionsOnIso", "cannot customize partitions when input is ISO")
	ErrInvalidBaseConfigs             = NewImageCustomizerError("Validation:InvalidBaseConfigs", "base configs contain invalid image config")
	ErrInvalidImageConfig             = NewImageCustomizerError("Validation:InvalidImageConfig", "invalid image config")
	ErrInvalidParameters              = NewImageCustomizerError("Validation:InvalidParameters", "invalid parameters")
	ErrVerityValidation               = NewImageCustomizerError("Validation:VerityValidation", "verity validation failed")
	ErrUkiReinitializeValidation      = NewImageCustomizerError("Validation:UkiReinitializeValidation", "UKI reinitialize validation failed")
	ErrUnsupportedQemuImageFormat     = NewImageCustomizerError("Validation:UnsupportedQemuImageFormat", "unsupported qemu-img format")
	ErrToolNotRunAsRoot               = NewImageCustomizerError("Validation:ToolNotRunAsRoot", "tool should be run as root (e.g. by using sudo)")
	ErrPackageSnapshotPreviewRequired = NewImageCustomizerError("Validation:PackageSnapshotPreviewRequired", fmt.Sprintf("preview feature '%s' required to specify package snapshot time", imagecustomizerapi.PreviewFeaturePackageSnapshotTime))
	ErrVerityPreviewFeatureRequired   = NewImageCustomizerError("Validation:VerityPreviewFeatureRequired", fmt.Sprintf("preview feature '%s' required to customize verity enabled base image", imagecustomizerapi.PreviewFeatureReinitializeVerity))
	ErrFedora42PreviewFeatureRequired = NewImageCustomizerError("Validation:Fedora42PreviewFeatureRequired", fmt.Sprintf("preview feature '%s' required to customize Fedora 42 base image", imagecustomizerapi.PreviewFeatureFedora42))
	ErrInputImageOciPreviewRequired   = NewImageCustomizerError("Validation:InputImageOciPreviewRequired", fmt.Sprintf("preview feature '%s' required to specify OCI input image", imagecustomizerapi.PreviewFeatureInputImageOci))

	// Generic customization errors
	ErrGetAbsoluteConfigPath    = NewImageCustomizerError("Customizer:GetAbsoluteConfigPath", "failed to get absolute path of config file directory")
	ErrCustomizeOs              = NewImageCustomizerError("Customizer:CustomizeOs", "failed to customize OS")
	ErrCustomizeProvisionVerity = NewImageCustomizerError("Customizer:ProvisionVerity", "failed to provision verity")
	ErrCustomizeCreateUkis      = NewImageCustomizerError("Customizer:CreateUkis", "failed to create UKIs")
	ErrCustomizeOutputArtifacts = NewImageCustomizerError("Customizer:OutputArtifacts", "failed to output artifacts")
	ErrCustomizeDownloadImage   = NewImageCustomizerError("Customizer:DownloadImage", "failed to download image")
	ErrOutputSelinuxPolicy      = NewImageCustomizerError("Customizer:OutputSelinuxPolicy", "failed to output SELinux policy")

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

type imageMetadata struct {
	baseImageVerityMetadata []verityDeviceMetadata
	verityMetadata          []verityDeviceMetadata

	partitionsLayout []fstabEntryPartNum
	osRelease        string
	osPackages       []OsPackage
	cosiBootMetadata *CosiBootloader
	targetOS         targetos.TargetOs
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

func CustomizeImageWithConfigFile(ctx context.Context, buildDir string, configFile string, inputImageFile string,
	rpmsSources []string, outputImageFile string, outputImageFormat string,
	useBaseImageRpmRepos bool, packageSnapshotTime string,
) error {
	return CustomizeImageWithConfigFileOptions(ctx, configFile, ImageCustomizerOptions{
		BuildDir:             buildDir,
		InputImageFile:       inputImageFile,
		RpmsSources:          rpmsSources,
		OutputImageFile:      outputImageFile,
		OutputImageFormat:    imagecustomizerapi.ImageFormatType(outputImageFormat),
		UseBaseImageRpmRepos: useBaseImageRpmRepos,
		PackageSnapshotTime:  imagecustomizerapi.PackageSnapshotTime(packageSnapshotTime),
	})
}

func CustomizeImageWithConfigFileOptions(ctx context.Context, configFile string, options ImageCustomizerOptions) error {
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

	err = CustomizeImageOptions(ctx, absBaseConfigPath, &config, options)
	if err != nil {
		return err
	}

	return nil
}

func cleanUp(rc *ResolvedConfig) error {
	err := file.RemoveFileIfExists(rc.RawImageFile)
	if err != nil {
		return err
	}

	return nil
}

func CustomizeImage(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.Config,
	inputImageFile string, rpmsSources []string, outputImageFile string, outputImageFormat string,
	useBaseImageRpmRepos bool, packageSnapshotTime string,
) (err error) {
	return CustomizeImageOptions(ctx, baseConfigPath, config, ImageCustomizerOptions{
		BuildDir:             buildDir,
		InputImageFile:       inputImageFile,
		RpmsSources:          rpmsSources,
		OutputImageFile:      outputImageFile,
		OutputImageFormat:    imagecustomizerapi.ImageFormatType(outputImageFormat),
		UseBaseImageRpmRepos: useBaseImageRpmRepos,
		PackageSnapshotTime:  imagecustomizerapi.PackageSnapshotTime(packageSnapshotTime),
	})
}

func CustomizeImageOptions(ctx context.Context, baseConfigPath string, config *imagecustomizerapi.Config,
	options ImageCustomizerOptions,
) (err error) {
	ctx, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "customize_image")
	span.SetAttributes(
		attribute.String("output_image_format", string(options.OutputImageFormat)),
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

	rc, err := ValidateConfig(ctx, baseConfigPath, config, false, options)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrInvalidImageConfig, err)
	}
	defer func() {
		cleanupErr := cleanUp(rc)
		if cleanupErr != nil {
			if err != nil {
				err = fmt.Errorf("%w:\nfailed to clean-up:\n%w", err, cleanupErr)
			} else {
				err = fmt.Errorf("failed to clean-up:\n%w", cleanupErr)
			}
		}
	}()

	// Ensure build and output folders are created up front
	err = os.MkdirAll(rc.BuildDirAbs, os.ModePerm)
	if err != nil {
		return err
	}

	outputImageDir := filepath.Dir(rc.OutputImageFile)
	err = os.MkdirAll(outputImageDir, os.ModePerm)
	if err != nil {
		return err
	}

	// Download base image (if neccessary)
	inputImageFilePath, err := downloadImage(ctx, rc.InputImage, options.BuildDir,
		options.ImageCacheDir)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrCustomizeDownloadImage, err)
	}

	rc.InputImage.Path = inputImageFilePath

	err = ValidateConfigPostImageDownload(rc)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrInvalidImageConfig, err)
	}

	err = CheckEnvironmentVars()
	if err != nil {
		return err
	}

	LogVersionsOfToolDeps()

	inputIsoArtifacts, err := convertInputImageToWriteableFormat(ctx, rc)
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

	im, err := customizeOSContents(ctx, rc)
	if err != nil {
		return err
	}

	if rc.OutputArtifacts != nil {
		outputDir := file.GetAbsPathWithBase(baseConfigPath, rc.OutputArtifacts.Path)

		err = outputArtifacts(ctx, rc.OutputArtifacts.Items, outputDir, rc.BuildDirAbs,
			rc.RawImageFile, im.verityMetadata)
		if err != nil {
			return fmt.Errorf("%w:\n%w", ErrCustomizeOutputArtifacts, err)
		}
	}

	if rc.OutputSelinuxPolicyPath != "" {
		err = outputSelinuxPolicy(ctx, rc.OutputSelinuxPolicyPath, rc.BuildDirAbs, rc.RawImageFile, im.partitionsLayout)
		if err != nil {
			return fmt.Errorf("%w:\n%w", ErrOutputSelinuxPolicy, err)
		}
	}

	err = convertWriteableFormatToOutputImage(ctx, rc, im, inputIsoArtifacts)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrConvertToOutputFormat, err)
	}

	logger.Log.Infof("Success!")

	return nil
}

func isKdumpBootFilesConfigChanging(requestedKdumpBootFiles *imagecustomizerapi.KdumpBootFilesType,
	inputKdumpBootFiles *imagecustomizerapi.KdumpBootFilesType,
) bool {
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

func convertInputImageToWriteableFormat(ctx context.Context, rc *ResolvedConfig) (*IsoArtifactsStore, error) {
	logger.Log.Infof("Converting input image to a writeable format")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "input_image_conversion")
	span.SetAttributes(
		attribute.String("input_image_format", rc.InputFileExt()),
	)
	defer span.End()

	if rc.InputIsIso() {
		inputIsoArtifacts, err := createIsoArtifactStoreFromIsoImage(rc.InputImage.Path,
			filepath.Join(rc.BuildDirAbs, "from-iso"))
		if err != nil {
			return inputIsoArtifacts, fmt.Errorf("%w (source='%s'):\n%w", ErrCreateArtifactsStore, rc.InputImage.Path, err)
		}

		var liveosConfig LiveOSConfig
		liveosConfig, convertInitramfsType, err := buildLiveOSConfig(inputIsoArtifacts, rc.Config.Iso,
			rc.Config.Pxe, rc.OutputImageFormat)
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
		rebuildFullOsImage := rc.CustomizeOSPartitions || convertInitramfsType || kdumpBootFileChanging

		if rebuildFullOsImage {
			err = createWriteableImageFromArtifacts(rc.BuildDirAbs, inputIsoArtifacts, rc.RawImageFile)
			if err != nil {
				return nil, fmt.Errorf("%w:\n%w", ErrCreateWriteableImage, err)
			}
		}

		return inputIsoArtifacts, nil
	} else {
		logger.Log.Infof("Creating raw base image: %s", rc.RawImageFile)

		_, err := convertImageToRaw(rc.InputImage.Path, rc.RawImageFile)
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

func customizeOSContents(ctx context.Context, rc *ResolvedConfig) (imageMetadata, error) {
	im := imageMetadata{}

	// If there are OS customizations, then we proceed as usual.
	// If there are no OS customizations, and the input is an iso, we just
	// return because this function is mainly about OS customizations.
	// This function also supports shrinking/exporting partitions. While
	// we could support those functions for input isos, we are choosing to
	// not support them until there is an actual need/a future time.
	// We explicitly inform the user of the lack of support earlier during
	// mic parameter validation (see createResolvedConfig()).
	if !rc.CustomizeOSPartitions && rc.InputIsIso() {
		return im, nil
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
	if rc.Config.OS == nil {
		rc.Config.OS = &imagecustomizerapi.OS{}
	}

	targetOS, err := validateTargetOs(ctx, rc)
	if err != nil {
		return im, fmt.Errorf("%w:\n%w", ErrCannotValidateTargetOS, err)
	}

	// Save target OS information
	im.targetOS = targetOS

	// Customize the partitions.
	partitionsCustomized, newRawImageFile, partIdToPartUuid, err := customizePartitions(ctx, rc.BuildDirAbs,
		rc.BaseConfigPath, rc.Config, rc.RawImageFile, im.targetOS)
	if err != nil {
		return im, err
	}

	if rc.RawImageFile != newRawImageFile {
		os.Remove(rc.RawImageFile)
		rc.RawImageFile = newRawImageFile
	}

	// Customize the raw image file.
	partitionsLayout, baseImageVerityMetadata, readonlyPartUuids, osRelease, err := customizeImageHelper(ctx, rc,
		partitionsCustomized, im.targetOS)
	if err != nil {
		return im, fmt.Errorf("%w:\n%w", ErrCustomizeOs, err)
	}

	if len(baseImageVerityMetadata) > 0 {
		previewFeatureEnabled := slices.Contains(rc.Config.PreviewFeatures,
			imagecustomizerapi.PreviewFeatureReinitializeVerity)
		if !previewFeatureEnabled {
			return im, ErrVerityPreviewFeatureRequired
		}
	}

	im.partitionsLayout = partitionsLayout
	im.baseImageVerityMetadata = baseImageVerityMetadata
	im.osRelease = osRelease

	// For COSI, always shrink the filesystems.
	shrinkPartitions := rc.OutputImageFormat == imagecustomizerapi.ImageFormatTypeCosi
	if shrinkPartitions {
		err = shrinkFilesystemsHelper(ctx, rc.RawImageFile, readonlyPartUuids)
		if err != nil {
			return im, fmt.Errorf("%w:\n%w", ErrShrinkFilesystems, err)
		}
	}

	if len(rc.Config.Storage.Verity) > 0 || len(im.baseImageVerityMetadata) > 0 {
		// Customize image for dm-verity, setting up verity metadata and security features.
		verityMetadata, err := customizeVerityImageHelper(ctx, rc.BuildDirAbs, rc.Config, rc.RawImageFile,
			partIdToPartUuid, shrinkPartitions, im.baseImageVerityMetadata, readonlyPartUuids, partitionsLayout)
		if err != nil {
			return im, fmt.Errorf("%w:\n%w", ErrCustomizeProvisionVerity, err)
		}

		im.verityMetadata = verityMetadata
	}

	if rc.Uki != nil {
		err = createUki(ctx, rc.BuildDirAbs, rc.RawImageFile, rc.Uki)
		if err != nil {
			return im, fmt.Errorf("%w:\n%w", ErrCustomizeCreateUkis, err)
		}
	}

	// collect OS info if generating a COSI image
	var osPackages []OsPackage
	var cosiBootMetadata *CosiBootloader
	if rc.Config.Output.Image.Format == imagecustomizerapi.ImageFormatTypeCosi || rc.OutputImageFormat == imagecustomizerapi.ImageFormatTypeCosi {
		osPackages, cosiBootMetadata, err = collectOSInfo(ctx, rc.BuildDirAbs, rc.RawImageFile, partitionsLayout)
		if err != nil {
			return im, fmt.Errorf("%w:\n%w", ErrCollectOSInfo, err)
		}
		im.osPackages = osPackages
		im.cosiBootMetadata = cosiBootMetadata
	}

	// Check file systems for corruption.
	err = checkFileSystems(ctx, rc.RawImageFile)
	if err != nil {
		return im, fmt.Errorf("%w:\n%w", ErrCheckFilesystems, err)
	}

	return im, nil
}

func convertWriteableFormatToOutputImage(ctx context.Context, rc *ResolvedConfig, im imageMetadata,
	inputIsoArtifacts *IsoArtifactsStore,
) error {
	logger.Log.Infof("Converting customized OS partitions into the final image")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "output_image_conversion")
	span.SetAttributes(
		attribute.String("output_image_format", string(rc.OutputImageFormat)),
	)
	defer span.End()

	// Create final output image file if requested.
	switch rc.OutputImageFormat {
	case imagecustomizerapi.ImageFormatTypeVhd, imagecustomizerapi.ImageFormatVhdTypeFixed,
		imagecustomizerapi.ImageFormatTypeVhdx, imagecustomizerapi.ImageFormatTypeQcow2,
		imagecustomizerapi.ImageFormatTypeRaw:
		logger.Log.Infof("Writing: %s", rc.OutputImageFile)

		err := ConvertImageFile(rc.RawImageFile, rc.OutputImageFile, rc.OutputImageFormat)
		if err != nil {
			return err
		}

	case imagecustomizerapi.ImageFormatTypeCosi:
		err := convertToCosi(rc.BuildDirAbs, rc.RawImageFile, rc.OutputImageFile, im.partitionsLayout,
			im.verityMetadata, im.osRelease, im.osPackages, rc.ImageUuid, rc.ImageUuidStr, im.cosiBootMetadata)
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
			liveosConfig, convertInitramfsType, err = buildLiveOSConfig(inputIsoArtifacts, rc.Config.Iso, rc.Config.Pxe,
				rc.OutputImageFormat)
			if err != nil {
				return fmt.Errorf("%w:\n%w", ErrBuildLiveOSConfig, err)
			}

			// Check if the user is changing the kdump boot files configuration.
			// If it is changing, it may change the composition of the full OS
			// image, and a reconstruction of the full OS image is needed.
			kdumpBootFileChanging = isKdumpBootFilesConfigChanging(liveosConfig.kdumpBootFiles, inputIsoArtifacts.info.kdumpBootFiles)
		}

		rebuildFullOsImage := rc.CustomizeOSPartitions || inputIsoArtifacts == nil || convertInitramfsType || kdumpBootFileChanging

		// Either re-build the full OS image, or just re-package the existing one
		if rebuildFullOsImage {
			requestedSELinuxMode := rc.SELinux.Mode
			err := createLiveOSFromRaw(ctx, rc.BuildDirAbs, rc.BaseConfigPath, inputIsoArtifacts, requestedSELinuxMode,
				rc.Config.Iso, rc.Config.Pxe, rc.RawImageFile, rc.OutputImageFormat, rc.OutputImageFile)
			if err != nil {
				return fmt.Errorf("%w:\n%w", ErrCreateLiveOSArtifacts, err)
			}
		} else {
			err := repackageLiveOS(rc.BuildDirAbs, rc.BaseConfigPath, rc.Config.Iso, rc.Config.Pxe,
				inputIsoArtifacts, rc.OutputImageFormat, rc.OutputImageFile)
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

func customizeImageHelper(ctx context.Context, rc *ResolvedConfig, partitionsCustomized bool,
	targetOS targetos.TargetOs,
) ([]fstabEntryPartNum, []verityDeviceMetadata, []string, string, error) {
	logger.Log.Debugf("Customizing OS")

	readOnlyVerity := rc.Config.Storage.ReinitializeVerity != imagecustomizerapi.ReinitializeVerityTypeAll

	imageConnection, partitionsLayout, baseImageVerityMetadata, readonlyPartUuids, err := connectToExistingImage(
		ctx, rc.RawImageFile, rc.BuildDirAbs, "imageroot", true, false, readOnlyVerity, false)
	if err != nil {
		return nil, nil, nil, "", err
	}
	defer imageConnection.Close()

	osRelease, err := extractOSRelease(imageConnection)
	if err != nil {
		return nil, nil, nil, "", err
	}

	// Create distro handler using the target OS determined earlier
	distroHandler := NewDistroHandlerFromTargetOs(targetOS)

	imageConnection.Chroot().UnsafeRun(func() error {
		distro, version := osinfo.GetDistroAndVersion()
		logger.Log.Infof("Base OS distro: %s", distro)
		logger.Log.Infof("Base OS version: %s", version)
		return nil
	})

	err = validateUkiReinitialize(imageConnection, rc.Config)
	if err != nil {
		return nil, nil, nil, "", err
	}

	err = validateVerityMountPaths(imageConnection, rc.Config, partitionsLayout, baseImageVerityMetadata)
	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("%w:\n%w", ErrVerityValidation, err)
	}

	// Do the actual customizations.
	err = doOsCustomizations(ctx, rc, imageConnection, partitionsCustomized, partitionsLayout, distroHandler)

	// Out of disk space errors can be difficult to diagnose.
	// So, warn about any partitions with low free space.
	warnOnLowFreeSpace(rc.BuildDirAbs, imageConnection)

	if err != nil {
		return nil, nil, nil, "", err
	}

	err = imageConnection.CleanClose()
	if err != nil {
		return nil, nil, nil, "", err
	}

	return partitionsLayout, baseImageVerityMetadata, readonlyPartUuids, osRelease, nil
}

func collectOSInfo(ctx context.Context, buildDir string, rawImageFile string, partitionsLayout []fstabEntryPartNum,
) ([]OsPackage, *CosiBootloader, error) {
	var err error
	imageConnection, _, err := reconnectToExistingImage(ctx, rawImageFile, buildDir, "imageroot", true, true, false,
		partitionsLayout)
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

// validateTargetOs checks if the current distro/version is supported and has the required preview
// features enabled. Returns the detected target OS.
func validateTargetOs(ctx context.Context, rc *ResolvedConfig,
) (targetos.TargetOs, error) {
	existingImageConnection, _, _, _, err := connectToExistingImage(ctx, rc.RawImageFile, rc.BuildDirAbs,
		"imageroot", false /* include-default-mounts */, true, /* read-only */
		false /* read-only-verity */, false /* ignore-overlays */)
	if err != nil {
		return "", err
	}
	defer existingImageConnection.Close()

	targetOs, err := targetos.GetInstalledTargetOs(existingImageConnection.Chroot().RootDir())
	if err != nil {
		return "", fmt.Errorf("failed to determine the target OS:\n%w", err)
	}

	// Check if Fedora 42 is being used and if it has the required preview feature
	if targetOs == targetos.TargetOsFedora42 {
		if !slices.Contains(rc.Config.PreviewFeatures, imagecustomizerapi.PreviewFeatureFedora42) {
			return targetOs, ErrFedora42PreviewFeatureRequired
		}

		hasPackageSnapshotTime := false

		if rc.Options.PackageSnapshotTime != "" {
			hasPackageSnapshotTime = true
		}

		if !hasPackageSnapshotTime {
			for _, configWithBase := range rc.ConfigChain {
				if configWithBase.Config.OS.Packages.SnapshotTime != "" {
					hasPackageSnapshotTime = true
					break
				}
			}
		}

		if hasPackageSnapshotTime {
			return targetOs, fmt.Errorf("Fedora 42 does not support package snapshotting:\n%w", ErrUnsupportedFedoraFeature)
		}
	}

	return targetOs, nil
}
