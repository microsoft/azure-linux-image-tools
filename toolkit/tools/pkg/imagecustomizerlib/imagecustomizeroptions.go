// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"slices"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
)

var (
	ErrInvalidInputImageStringFormat  = NewImageCustomizerError("Validation:InvalidInputImageStringFormat", "invalid --image string format")
	ErrMultipleInputImageOptions      = NewImageCustomizerError("Validation:MultipleInputImageOptions", "cannot specify both --image and --image-file")
	ErrInvalidCosiCompressionLevelArg = NewImageCustomizerError("Validation:InvalidCosiCompressionLevelArg", "invalid --cosi-compression-level value")
)

type ImageCustomizerOptions struct {
	BuildDir                string
	InputImageFile          string
	InputImage              string
	RpmsSources             []string
	OutputImageFile         string
	OutputImageFormat       imagecustomizerapi.ImageFormatType
	OutputSelinuxPolicyPath string
	UseBaseImageRpmRepos    bool
	PackageSnapshotTime     imagecustomizerapi.PackageSnapshotTime
	ImageCacheDir           string
	CosiCompressionLevel    *int
	PreviewFeatures         []imagecustomizerapi.PreviewFeature
}

func (o *ImageCustomizerOptions) IsValid() error {
	if err := o.OutputImageFormat.IsValid(); err != nil {
		return fmt.Errorf("%w (format='%s'):\n%w", ErrInvalidOutputFormat, o.OutputImageFormat, err)
	}

	if err := o.PackageSnapshotTime.IsValid(); err != nil {
		return fmt.Errorf("%w (time='%s'):\n%w", ErrInvalidPackageSnapshotTime, o.PackageSnapshotTime, err)
	}

	if o.InputImage != "" {
		if _, err := parseInputImage(o.InputImage); err != nil {
			return err
		}
	}

	if o.InputImageFile != "" && o.InputImage != "" {
		return ErrMultipleInputImageOptions
	}

	if o.CosiCompressionLevel != nil &&
		(*o.CosiCompressionLevel < imagecustomizerapi.MinCosiCompressionLevel ||
			*o.CosiCompressionLevel > imagecustomizerapi.MaxCosiCompressionLevel) {
		return fmt.Errorf("%w (level=%d, valid range: %d-%d)",
			ErrInvalidCosiCompressionLevelArg, *o.CosiCompressionLevel,
			imagecustomizerapi.MinCosiCompressionLevel, imagecustomizerapi.MaxCosiCompressionLevel)
	}

	return nil
}

func (o *ImageCustomizerOptions) verifyPreviewFeatures(previewFeatures []imagecustomizerapi.PreviewFeature) error {
	if o.PackageSnapshotTime != "" {
		if !slices.Contains(previewFeatures, imagecustomizerapi.PreviewFeaturePackageSnapshotTime) {
			return ErrPackageSnapshotPreviewRequired
		}
	}

	if o.InputImage != "" {
		if !slices.Contains(previewFeatures, imagecustomizerapi.PreviewFeatureInputImageOci) {
			return ErrInputImageOciPreviewRequired
		}
	}

	if o.CosiCompressionLevel != nil {
		if !slices.Contains(previewFeatures, imagecustomizerapi.PreviewFeatureCosiCompression) {
			return ErrCosiCompressionPreviewRequired
		}
	}

	return nil
}

func parseInputImage(inputImageStr string) (imagecustomizerapi.InputImage, error) {
	inputImage, err := parseInputImageHelper(inputImageStr)
	if err != nil {
		err = fmt.Errorf("%w (value='%s'):\n%w:\nsupported formats:\n- oci:<URI>\n- azurelinux:<VARIANT>:<VERSION>",
			ErrInvalidInputImageStringFormat, inputImageStr, err)
		return inputImage, err
	}

	return inputImage, nil
}

func parseInputImageHelper(inputImage string) (imagecustomizerapi.InputImage, error) {
	resourceType, value, found := strings.Cut(inputImage, ":")
	if !found {
		err := fmt.Errorf("resource type not found")
		return imagecustomizerapi.InputImage{}, err
	}

	switch resourceType {
	case "oci":
		inputImage := imagecustomizerapi.InputImage{
			Oci: &imagecustomizerapi.OciImage{
				Uri: value,
			},
		}

		err := inputImage.Oci.IsValid()
		if err != nil {
			return imagecustomizerapi.InputImage{}, err
		}

		return inputImage, nil

	case "azurelinux":
		return parseInputImageAzureLinuxValue(value)

	default:
		err := fmt.Errorf("unknown resource type (%s)", resourceType)
		return imagecustomizerapi.InputImage{}, err
	}
}
