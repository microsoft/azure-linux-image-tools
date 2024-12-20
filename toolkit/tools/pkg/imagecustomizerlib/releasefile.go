// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
)

func addCustomizerRelease(rootDir string, toolVersion string, buildTime string, imageUuid string) error {
	var err error

	logger.Log.Infof("Creating image customizer release file")

	customizerReleaseFilePath := filepath.Join(rootDir, "/etc/image-customizer-release")
	lines := []string{
		fmt.Sprintf("%s=\"%s\"", "TOOL_VERSION", toolVersion),
		fmt.Sprintf("%s=\"%s\"", "BUILD_DATE", buildTime),
		fmt.Sprintf("%s=\"%s\"", "IMAGE_UUID", imageUuid),
		"",
	}
	err = file.WriteLines(lines, customizerReleaseFilePath)
	if err != nil {
		return fmt.Errorf("error writing customizer release file (%s): %w", customizerReleaseFilePath, err)
	}

	return nil
}
