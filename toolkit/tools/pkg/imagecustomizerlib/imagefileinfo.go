// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"encoding/json"
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
)

var (
	// Image file info errors
	ErrImageFormatCheck = NewImageCustomizerError("ImageInfo:FormatCheck", "failed to check image file's disk format")
	ErrQemuImgInfo      = NewImageCustomizerError("ImageInfo:QemuImgInfo", "failed to parse qemu-img info JSON")
)

type ImageFileInfo struct {
	Format      string `json:"format"`
	VirtualSize int64  `json:"virtual-size"`
}

func GetImageFileInfo(inputImageFile string) (ImageFileInfo, error) {
	stdout, _, err := shell.Execute("qemu-img", "info", "--output", "json", inputImageFile)
	if err != nil {
		return ImageFileInfo{}, fmt.Errorf("%w:\n%w", ErrImageFormatCheck, err)
	}

	info := ImageFileInfo{}
	err = json.Unmarshal([]byte(stdout), &info)
	if err != nil {
		return ImageFileInfo{}, fmt.Errorf("%w:\n%w", ErrQemuImgInfo, err)
	}

	return info, nil
}
