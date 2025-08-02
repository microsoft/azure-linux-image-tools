// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var (
	// File operation errors
	ErrFileCopy      = NewImageCustomizerError("Files:Copy", "failed to copy file")
	ErrDirectoryCopy = NewImageCustomizerError("Files:DirectoryCopy", "failed to copy directory")
)

const (
	defaultFilePermissions = 0o755
)

func copyAdditionalFiles(ctx context.Context, baseConfigPath string, additionalFiles imagecustomizerapi.AdditionalFileList,
	imageChroot *safechroot.Chroot,
) error {

	if len(additionalFiles) == 0 {
		return nil
	}
	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "copy_additional_files")
	span.SetAttributes(
		attribute.Int("files_count", len(additionalFiles)),
	)
	defer span.End()

	for _, additionalFile := range additionalFiles {
		logger.Log.Infof("Copying: %s", additionalFile.Destination)

		absSourceFile := ""
		if additionalFile.Source != "" {
			absSourceFile = file.GetAbsPathWithBase(baseConfigPath, additionalFile.Source)
		}

		fileToCopy := safechroot.FileToCopy{
			Src:         absSourceFile,
			Content:     additionalFile.Content,
			Dest:        additionalFile.Destination,
			Permissions: (*fs.FileMode)(additionalFile.Permissions),
		}

		err := imageChroot.AddFiles(fileToCopy)
		if err != nil {
			return fmt.Errorf("%w (destination='%s'):\n%w", ErrFileCopy, additionalFile.Destination, err)
		}
	}

	return nil
}

func copyAdditionalDirs(ctx context.Context, baseConfigPath string, additionalDirs imagecustomizerapi.DirConfigList, imageChroot *safechroot.Chroot) error {
	if len(additionalDirs) == 0 {
		return nil
	}
	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "copy_additional_directories")
	span.SetAttributes(
		attribute.Int("directories_count", len(additionalDirs)),
	)
	defer span.End()
	for _, dirConfigElement := range additionalDirs {
		absSourceDir := file.GetAbsPathWithBase(baseConfigPath, dirConfigElement.Source)
		logger.Log.Infof("Copying %s to %s", absSourceDir, dirConfigElement.Destination)

		// Setting permissions values. They are set to a default value if they have not been specified.
		newDirPermissionsValue := fs.FileMode(defaultFilePermissions)
		if dirConfigElement.NewDirPermissions != nil {
			newDirPermissionsValue = *(*fs.FileMode)(dirConfigElement.NewDirPermissions)
		}
		childFilePermissionsValue := fs.FileMode(defaultFilePermissions)
		if dirConfigElement.ChildFilePermissions != nil {
			childFilePermissionsValue = *(*fs.FileMode)(dirConfigElement.ChildFilePermissions)
		}

		dirToCopy := safechroot.DirToCopy{
			Src:                  absSourceDir,
			Dest:                 dirConfigElement.Destination,
			NewDirPermissions:    newDirPermissionsValue,
			ChildFilePermissions: childFilePermissionsValue,
			MergedDirPermissions: (*fs.FileMode)(dirConfigElement.MergedDirPermissions),
		}
		err := imageChroot.AddDirs(dirToCopy)
		if err != nil {
			return fmt.Errorf("%w (source='%s', destination='%s'):\n%w", ErrDirectoryCopy, absSourceDir, dirConfigElement.Destination, err)
		}
	}
	return nil
}
