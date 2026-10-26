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
	"go.opentelemetry.io/otel"
)

var (
	ErrPartitionsCustomize  = NewImageCustomizerError("Partitions:Customize", "failed to customize partitions")
	ErrPartitionsResetUuids = NewImageCustomizerError("Partitions:ResetUuids", "failed to reset partition UUIDs")
	ErrResizeDisk           = NewImageCustomizerError("Partitions:ResizeDisk", "failed to resize disk and partitions")
)

// Handles any partition customization requested.
//
// Returns
// - partitionsCustomized: True if the partitions' metadata was changed/updated.
// - partIdToPartUuid: Mapping from config partition ID to partition UUID.
// - osImageConnection: If not nil, a connection to the new OS image, with the chroot disconnected.
func customizePartitions(ctx context.Context, buildDir string, storage imagecustomizerapi.Storage,
	osImageConnection *osImageConnection, distroHandler DistroHandler,
) (bool, map[string]string, *osImageConnection, error) {
	switch {
	case storage.CustomizePartitions():
		logger.Log.Infof("Customizing partitions")

		_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "customize_partitions")
		defer span.End()

		newBuildImageFile := filepath.Join(buildDir, PartitionCustomizedImageName)

		partIdToPartUuid, newOsImageConnection, err := customizePartitionsUsingFileCopy(ctx, buildDir, storage,
			osImageConnection, newBuildImageFile, distroHandler)
		if err != nil {
			os.Remove(newBuildImageFile)
			return false, nil, nil, fmt.Errorf("%w:\n%w", ErrPartitionsCustomize, err)
		}

		return true, partIdToPartUuid, newOsImageConnection, nil

	case storage.ResetPartitionsUuidsType != imagecustomizerapi.ResetPartitionsUuidsTypeDefault:
		err := resetPartitionsUuids(ctx, osImageConnection, buildDir, distroHandler)
		if err != nil {
			return false, nil, nil, fmt.Errorf("%w:\n%w", ErrPartitionsResetUuids, err)
		}

		return true, nil, nil, nil

	case storage.ResizeDisk != nil:
		err := resizeDiskAndPartitions(ctx, osImageConnection, buildDir, *storage.ResizeDisk)
		if err != nil {
			return false, nil, nil, fmt.Errorf("%w:\n%w", ErrResizeDisk, err)
		}

		return false, nil, nil, nil

	default:
		// No changes to make to the partitions.
		// So, just use the original disk.
		return false, nil, nil, nil
	}
}
