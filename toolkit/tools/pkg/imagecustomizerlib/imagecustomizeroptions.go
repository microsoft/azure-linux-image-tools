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
	ErrInvalidInputImageStringFormat = NewImageCustomizerError("Validation:InvalidInputImageStringFormat", "invalid --image string format")
	ErrMultipleInputImageOptions     = NewImageCustomizerError("Validation:MultipleInputImageOptions", "cannot specify both --image and --image-file")
)

type ImageCustomizerOptions struct {
	BuildDir             string
	InputImageFile       string
	InputImage           string
	RpmsSources          []string
	OutputImageFile      string
	OutputImageFormat    imagecustomizerapi.ImageFormatType
	UseBaseImageRpmRepos bool
	PackageSnapshotTime  imagecustomizerapi.PackageSnapshotTime
	ImageCacheDir        string
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

	return nil
}

func parseInputImage(inputImage string) (imagecustomizerapi.OciImage, error) {
	uri, isOci := strings.CutPrefix(inputImage, "oci:")
	if isOci {
		ociImage := imagecustomizerapi.OciImage{
			Uri: uri,
		}

		err := ociImage.IsValid()
		if err != nil {
			err = fmt.Errorf("%w\n%w", ErrInvalidInputImageStringFormat, err)
			return imagecustomizerapi.OciImage{}, err
		}

		return ociImage, nil
	}

	err := fmt.Errorf("%w (value='%s'):\nSupported formats:\n- oci:<URI>", ErrInvalidInputImageStringFormat, inputImage)
	return imagecustomizerapi.OciImage{}, err
}
