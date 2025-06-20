// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"encoding/json"
	"fmt"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
)

type ImageFileInfo struct {
	Format      string `json:"format"`
	VirtualSize int64  `json:"virtual-size"`
}

func GetImageFileInfo(inputImageFile string) (ImageFileInfo, error) {
	stdout, _, err := shell.Execute("qemu-img", "info", "--output", "json", inputImageFile)
	if err != nil {
		return ImageFileInfo{}, fmt.Errorf("failed to check image file's disk format:\n%w", err)
	}

	info := ImageFileInfo{}
	err = json.Unmarshal([]byte(stdout), &info)
	if err != nil {
		return ImageFileInfo{}, fmt.Errorf("failed to qemu-img info JSON:\n%w", err)
	}

	return info, nil
}
