// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"go.opentelemetry.io/otel"
)

var (
	// Release file errors
	ErrReleaseFileWrite = NewImageCustomizerError("ReleaseFile:Write", "failed to write customizer release file")
)

const (
	ImageCustomizerReleasePath = "etc/image-customizer-release"
)

func addCustomizerRelease(ctx context.Context, rootDir string, toolVersion string, buildTime string, imageUuid string) error {
	var err error

	logger.Log.Infof("Creating image customizer release file")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "add_customizer_release")
	defer span.End()

	customizerReleaseFilePath := filepath.Join(rootDir, ImageCustomizerReleasePath)
	lines := []string{
		fmt.Sprintf("%s=\"%s\"", "TOOL_VERSION", toolVersion),
		fmt.Sprintf("%s=\"%s\"", "BUILD_DATE", buildTime),
		fmt.Sprintf("%s=\"%s\"", "IMAGE_UUID", imageUuid),
		"",
	}
	err = file.WriteLines(lines, customizerReleaseFilePath)
	if err != nil {
		return fmt.Errorf("%w (path='%s'):\n%w", ErrReleaseFileWrite, customizerReleaseFilePath, err)
	}

	return nil
}
