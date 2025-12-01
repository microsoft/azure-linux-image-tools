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
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	"go.opentelemetry.io/otel"
)

var (
	ErrPartitionsCustomize  = NewImageCustomizerError("Partitions:Customize", "failed to customize partitions")
	ErrPartitionsResetUuids = NewImageCustomizerError("Partitions:ResetUuids", "failed to reset partition UUIDs")
)

func customizePartitions(ctx context.Context, rc *ResolvedConfig, targetOS targetos.TargetOs,
) (bool, string, map[string]string, error) {

	storage := rc.Storage

	switch {
	case storage.CustomizePartitions():
		logger.Log.Infof("Customizing partitions")

		_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "customize_partitions")
		defer span.End()

		newBuildImageFile := filepath.Join(rc.BuildDirAbs, PartitionCustomizedImageName)

		// If there is no known way to create the new partition layout from the old one,
		// then fallback to creating the new partitions from scratch and doing a file copy.
		partIdToPartUuid, err := customizePartitionsUsingFileCopy(ctx, rc, newBuildImageFile, targetOS)
		if err != nil {
			os.Remove(newBuildImageFile)
			return false, "", nil, fmt.Errorf("%w:\n%w", ErrPartitionsCustomize, err)
		}

		return true, newBuildImageFile, partIdToPartUuid, nil

	case storage.ResetPartitionsUuidsType != imagecustomizerapi.ResetPartitionsUuidsTypeDefault:
		err := resetPartitionsUuids(ctx, rc.RawImageFile, rc.BuildDirAbs)
		if err != nil {
			return false, "", nil, fmt.Errorf("%w:\n%w", ErrPartitionsResetUuids, err)
		}

		return true, rc.RawImageFile, nil, nil

	default:
		// No changes to make to the partitions.
		// So, just use the original disk.
		return false, rc.RawImageFile, nil, nil
	}
}
