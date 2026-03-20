// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"slices"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"go.opentelemetry.io/otel"
)

func resizeDiskAndPartitions(ctx context.Context, buildImageFile string, buildDir string,
	resizeConfig imagecustomizerapi.ResizeDisk,
) error {
	if len(resizeConfig.Partitions) <= 0 {
		// Nothing to do.
		return nil
	}

	logger.Log.Infof("Resize disk and partitions")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "resize_disk")
	defer span.End()

	loopback, err := safeloopback.NewLoopback(buildImageFile)
	if err != nil {
		return err
	}
	defer loopback.Close()

	err = resizeDiskAndPartitionsHelper(loopback, buildDir, resizeConfig)
	if err != nil {
		return err
	}

	err = loopback.CleanClose()
	if err != nil {
		return err
	}

	return nil
}

func resizeDiskAndPartitionsHelper(loopback *safeloopback.Loopback, buildDir string,
	resizeConfig imagecustomizerapi.ResizeDisk,
) error {
	partitionTable, err := diskutils.ReadDiskPartitionTable(loopback.DevicePath())
	if err != nil {
		return err
	}

	partitionConfig := resizeConfig.Partitions[0]
	requiredFreeSpace := uint64(*partitionConfig.FreeSpace) // Pointer checked by ResizePartition.IsValid()

	lastPartition := slices.MaxFunc(partitionTable.Partitions,
		func(a diskutils.PartitionTablePartition, b diskutils.PartitionTablePartition) int {
			switch {
			case a.Start > b.Start:
				return 1
			case a.Start == b.Start:
				return 0
			default:
				return -1
			}
		})

	currentFreeSpace, err := getPartitionFreeSpace(lastPartition.Path)
	if err != nil {
		return err
	}

	if currentFreeSpace >= requiredFreeSpace {
		// Nothing to do.
		return nil
	}

	missingFreeSpace := requiredFreeSpace - currentFreeSpace
	newSize := uint64(lastPartition.Size) + missingFreeSpace
	newSize = diskutils.RoundUp(newSize, imagecustomizerapi.DefaultPartitionAlignment)

	return nil
}

func getPartitionFreeSpace(partitionPath string) (uint64, error) {

}
