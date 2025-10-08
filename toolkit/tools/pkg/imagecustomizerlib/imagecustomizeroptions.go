// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
)

type ImageCustomizerOptions struct {
	BuildDir             string
	InputImageFile       string
	RpmsSources          []string
	OutputImageFile      string
	OutputImageFormat    imagecustomizerapi.ImageFormatType
	UseBaseImageRpmRepos bool
	PackageSnapshotTime  imagecustomizerapi.PackageSnapshotTime
}

func (o *ImageCustomizerOptions) IsValid() error {
	if err := o.OutputImageFormat.IsValid(); err != nil {
		return fmt.Errorf("%w (format='%s'):\n%w", ErrInvalidOutputFormat, o.OutputImageFormat, err)
	}

	if err := o.PackageSnapshotTime.IsValid(); err != nil {
		return fmt.Errorf("%w (time='%s'):\n%w", ErrInvalidPackageSnapshotTime, o.PackageSnapshotTime, err)
	}

	return nil
}
