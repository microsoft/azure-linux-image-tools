// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"os"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"go.opentelemetry.io/otel"
)

func customizePartitions(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.Config,
	buildImageFile string,
) (bool, string, map[string]string, error) {
	switch {
	case config.CustomizePartitions():
		logger.Log.Infof("Customizing partitions")

		_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "customize_partitions")
		defer span.End()

		newBuildImageFile := filepath.Join(buildDir, PartitionCustomizedImageName)

		// If there is no known way to create the new partition layout from the old one,
		// then fallback to creating the new partitions from scratch and doing a file copy.
		partIdToPartUuid, err := customizePartitionsUsingFileCopy(ctx, buildDir, baseConfigPath, config,
			buildImageFile, newBuildImageFile)
		if err != nil {
			os.Remove(newBuildImageFile)
			return false, "", nil, err
		}

		return true, newBuildImageFile, partIdToPartUuid, nil

	case config.Storage.ResetPartitionsUuidsType != imagecustomizerapi.ResetPartitionsUuidsTypeDefault:
		err := resetPartitionsUuids(ctx, buildImageFile, buildDir)
		if err != nil {
			return false, "", nil, err
		}

		return true, buildImageFile, nil, nil

	default:
		// No changes to make to the partitions.
		// So, just use the original disk.
		return false, buildImageFile, nil, nil
	}
}
