// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

func ConvertImage(ctx context.Context, options ConvertImageOptions) (err error) {
	logger.Log.Infof("Converting image from one format to another")

	ctx, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "convert_image")
	span.SetAttributes(
		attribute.String("output_image_format", string(options.OutputImageFormat)),
	)
	if options.CosiCompressionLevel != nil {
		span.SetAttributes(attribute.Int("cosi_compression_level", *options.CosiCompressionLevel))
	}
	defer finishSpanWithError(span, &err)

	if err := options.IsValid(); err != nil {
		return err
	}

	// Validate input image file exists and is a file
	if isFile, err := file.IsFile(options.InputImageFile); err != nil {
		return fmt.Errorf("%w (file='%s'):\n%w", ErrInvalidInputImageFileArg, options.InputImageFile, err)
	} else if !isFile {
		return fmt.Errorf("%w (file='%s')", ErrInputImageFileNotFile, options.InputImageFile)
	}

	// Validate output image file is not a directory
	if isDir, err := file.DirExists(options.OutputImageFile); err != nil {
		return fmt.Errorf("%w (file='%s'):\n%w", ErrInvalidOutputImageFileArg, options.OutputImageFile, err)
	} else if isDir {
		return fmt.Errorf("%w (file='%s')", ErrOutputImageFileIsDirectory, options.OutputImageFile)
	}

	outputFormat := options.OutputImageFormat

	// Detect input image format and reject unsupported formats
	inputImageInfo, err := GetImageFileInfo(options.InputImageFile)
	if err != nil {
		return fmt.Errorf("%w (file='%s'):\n%w", ErrDetectImageFormat, options.InputImageFile, err)
	}

	// Validate the detected format is a supported input format
	inputFormat, err := qemuStringtoImageFormatType(inputImageInfo.Format)
	if err != nil {
		return fmt.Errorf("%w (format='%s'):\n%w", ErrConvertUnsupportedInputFormat, inputImageInfo.Format, err)
	}
	if inputFormat == imagecustomizerapi.ImageFormatTypeIso {
		return fmt.Errorf("%w (format='%s')", ErrConvertUnsupportedInputFormat, inputImageInfo.Format)
	}

	// Add input image format attribute after detection
	span.SetAttributes(attribute.String("input_image_format", inputImageInfo.Format))

	isCosiOutput := outputFormat == imagecustomizerapi.ImageFormatTypeCosi ||
		outputFormat == imagecustomizerapi.ImageFormatTypeBareMetalImage
	if isCosiOutput {
		err = convertImageToCosi(ctx, options.BuildDir, options.InputImageFile, options.OutputImageFile,
			outputFormat, options.CosiCompressionLevel)
		if err != nil {
			return err
		}
	} else {
		err = ConvertImageFileFromAnyFormat(options.InputImageFile, options.OutputImageFile, outputFormat)
		if err != nil {
			return fmt.Errorf("%w (output='%s', format='%s'):\n%w", ErrArtifactOutputImageConversion,
				options.OutputImageFile, outputFormat, err)
		}
	}

	logger.Log.Infof("Success!")

	return nil
}

func convertImageToCosi(ctx context.Context, buildDir string, inputImageFile string, outputImageFile string,
	outputFormat imagecustomizerapi.ImageFormatType, cosiCompressionLevel *int,
) error {
	buildDirAbs, err := filepath.Abs(buildDir)
	if err != nil {
		return err
	}

	err = os.MkdirAll(buildDirAbs, os.ModePerm)
	if err != nil {
		return err
	}

	rawImageFile := filepath.Join(buildDirAbs, BaseImageName)
	_, err = convertImageToRaw(inputImageFile, rawImageFile)
	if err != nil {
		return err
	}

	err = convertRawImageToOutputFormat(ctx, buildDirAbs, rawImageFile, outputFormat,
		outputImageFile, cosiCompressionLevel)
	if err != nil {
		return err
	}

	return nil
}

func convertRawImageToOutputFormat(ctx context.Context, buildDirAbs string, rawImageFile string,
	outputFormat imagecustomizerapi.ImageFormatType, outputImageFile string, cosiCompressionLevel *int,
) error {
	if outputFormat == imagecustomizerapi.ImageFormatTypeCosi || outputFormat == imagecustomizerapi.ImageFormatTypeBareMetalImage {
		// Convert subcommand doesn't support preview features - pass empty slice
		var previewFeatures []imagecustomizerapi.PreviewFeature
		partitionsLayout, baseImageVerityMetadata, osRelease, osPackages, imageUuid, imageUuidStr, cosiBootMetadata,
			readonlyPartUuids, err := prepareImageConversionData(ctx, rawImageFile, buildDirAbs, "imageroot", previewFeatures)
		if err != nil {
			return err
		}

		// For convert subcommand, we're dealing with arbitrary external images.
		// Only shrink filesystems that completely cover their partition.
		partitionOriginalSizes, err := shrinkFilesystemsHelper(ctx, rawImageFile, readonlyPartUuids, true /*isExternalImage*/)
		if err != nil {
			return fmt.Errorf("%w:\n%w", ErrShrinkFilesystems, err)
		}

		compressionLevel := defaultCosiCompressionLevel(outputFormat)
		if cosiCompressionLevel != nil {
			compressionLevel = *cosiCompressionLevel
		}
		compressionLong := defaultCosiCompressionLong(outputFormat)

		includeVhdFooter := outputFormat == imagecustomizerapi.ImageFormatTypeBareMetalImage

		err = convertToCosi(buildDirAbs, rawImageFile, outputImageFile, partitionsLayout,
			baseImageVerityMetadata, osRelease, osPackages, imageUuid, imageUuidStr, cosiBootMetadata,
			compressionLevel, compressionLong, includeVhdFooter, partitionOriginalSizes)
		if err != nil {
			return fmt.Errorf("%w (output='%s'):\n%w", ErrArtifactCosiImageConversion, outputImageFile, err)
		}
	} else {
		err := ConvertImageFile(rawImageFile, outputImageFile, outputFormat)
		if err != nil {
			return fmt.Errorf("%w (output='%s', format='%s'):\n%w", ErrArtifactOutputImageConversion, outputImageFile,
				outputFormat, err)
		}
	}

	return nil
}
