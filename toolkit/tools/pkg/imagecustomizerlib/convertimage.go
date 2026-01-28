// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"go.opentelemetry.io/otel"
)

func ConvertImageWithOptions(ctx context.Context, options ConvertImageOptions) error {
	logger.Log.Infof("Converting image from one format to another")

	ctx, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "convert_image")
	defer span.End()

	if err := options.IsValid(); err != nil {
		return err
	}

	buildDirAbs, err := filepath.Abs(options.BuildDir)
	if err != nil {
		return err
	}

	err = os.MkdirAll(buildDirAbs, os.ModePerm)
	if err != nil {
		return err
	}

	// OutputImageFormat is required, so no need to fall back to detected format
	outputFormat := imagecustomizerapi.ImageFormatType(options.OutputImageFormat)

	if options.CosiCompressionLevel != nil {
		if outputFormat != imagecustomizerapi.ImageFormatTypeCosi &&
			outputFormat != imagecustomizerapi.ImageFormatTypeBareMetalImage {
			return fmt.Errorf("COSI compression level can only be specified for COSI or bare-metal-image output formats")
		}
	}

	err = options.verifyPreviewFeatures(options.PreviewFeatures)
	if err != nil {
		return err
	}

	if outputFormat == imagecustomizerapi.ImageFormatTypeCosi || outputFormat == imagecustomizerapi.ImageFormatTypeBareMetalImage {
		rawImageFile := filepath.Join(buildDirAbs, BaseImageName)
		_, err = convertImageToRaw(options.InputImageFile, rawImageFile)
		if err != nil {
			return err
		}

		err = convertRawImageToOutputFormat(ctx, buildDirAbs, rawImageFile, outputFormat,
			options.OutputImageFile, options.CosiCompressionLevel, options.PreviewFeatures)
		if err != nil {
			return err
		}
	} else {
		err = ConvertImageFile(options.InputImageFile, options.OutputImageFile, outputFormat)
		if err != nil {
			return fmt.Errorf("%w (output='%s', format='%s'):\n%w", ErrArtifactOutputImageConversion,
				options.OutputImageFile, outputFormat, err)
		}
	}

	logger.Log.Infof("Success!")

	return nil
}

func convertRawImageToOutputFormat(ctx context.Context, buildDirAbs string, rawImageFile string,
	outputFormat imagecustomizerapi.ImageFormatType, outputImageFile string, cosiCompressionLevel *int,
	previewFeatures []imagecustomizerapi.PreviewFeature,
) error {
	if outputFormat == imagecustomizerapi.ImageFormatTypeCosi || outputFormat == imagecustomizerapi.ImageFormatTypeBareMetalImage {
		partitionsLayout, baseImageVerityMetadata, osRelease, osPackages, imageUuid, imageUuidStr, cosiBootMetadata,
			readonlyPartUuids, err := prepareImageConversionData(ctx, rawImageFile, buildDirAbs, "imageroot", previewFeatures)
		if err != nil {
			return err
		}

		partitionOriginalSizes, err := shrinkFilesystemsHelper(ctx, rawImageFile, readonlyPartUuids)
		if err != nil {
			return fmt.Errorf("%w:\n%w", ErrShrinkFilesystems, err)
		}

		compressionLevel := imagecustomizerapi.DefaultCosiCompressionLevel
		if cosiCompressionLevel != nil {
			compressionLevel = *cosiCompressionLevel
		}

		includeVhdFooter := outputFormat == imagecustomizerapi.ImageFormatTypeBareMetalImage

		err = convertToCosi(buildDirAbs, rawImageFile, outputImageFile, partitionsLayout,
			baseImageVerityMetadata, osRelease, osPackages, imageUuid, imageUuidStr, cosiBootMetadata,
			compressionLevel, imagecustomizerapi.DefaultCosiCompressionLong, includeVhdFooter, partitionOriginalSizes)
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
