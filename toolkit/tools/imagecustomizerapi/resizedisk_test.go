// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/ptrutils"
	"github.com/stretchr/testify/assert"
)

func TestResizeDiskIsValidEmpty(t *testing.T) {
	script := ResizeDisk{}
	err := script.IsValid()
	assert.ErrorContains(t, err, "must specify either diskSize or partitions")
}

func TestResizeDiskIsValidBothDiskSizeAndPartitions(t *testing.T) {
	script := ResizeDisk{
		Partitions: []ResizePartition{
			{
				Ref:       ResizePartitionRefLast,
				FreeSpace: ptrutils.PtrTo(DiskSize(1 * diskutils.GiB)),
			},
		},
		DiskSize: ptrutils.PtrTo(DiskSize(1 * diskutils.GiB)),
	}
	err := script.IsValid()
	assert.ErrorContains(t, err, "cannot specify both diskSize and partitions")
}

func TestResizeDiskIsValidDiskSize(t *testing.T) {
	script := ResizeDisk{
		DiskSize: ptrutils.PtrTo(DiskSize(1 * diskutils.GiB)),
	}
	err := script.IsValid()
	assert.NoError(t, err)
}

func TestResizeDiskIsValidLastNoSize(t *testing.T) {
	script := ResizeDisk{
		Partitions: []ResizePartition{
			{
				Ref: ResizePartitionRefLast,
			},
		},
	}
	err := script.IsValid()
	assert.ErrorContains(t, err, "one of 'freeSpace' or 'size' must be specified")
}

func TestResizeDiskIsValidLastBothSizeAndFreeSpace(t *testing.T) {
	script := ResizeDisk{
		Partitions: []ResizePartition{
			{
				Ref:       ResizePartitionRefLast,
				FreeSpace: ptrutils.PtrTo(DiskSize(1 * diskutils.GiB)),
				Size:      ptrutils.PtrTo(DiskSize(1 * diskutils.GiB)),
			},
		},
	}
	err := script.IsValid()
	assert.ErrorContains(t, err, "cannot specify both 'freeSpace' and 'size'")
}

func TestResizeDiskIsValidLastFreeSpace(t *testing.T) {
	script := ResizeDisk{
		Partitions: []ResizePartition{
			{
				Ref:       ResizePartitionRefLast,
				FreeSpace: ptrutils.PtrTo(DiskSize(1 * diskutils.GiB)),
			},
		},
	}
	err := script.IsValid()
	assert.NoError(t, err)
}

func TestResizeDiskIsValidInvalidRef(t *testing.T) {
	script := ResizeDisk{
		Partitions: []ResizePartition{
			{
				Ref:       "test",
				FreeSpace: ptrutils.PtrTo(DiskSize(1 * diskutils.GiB)),
			},
		},
	}
	err := script.IsValid()
	assert.ErrorContains(t, err, "invalid 'partitions' value at index 0:\n")
	assert.ErrorContains(t, err, "invalid 'ref' value:\n")
	assert.ErrorContains(t, err, "invalid value (test)")
}

func TestResizeDiskIsValidLastSize(t *testing.T) {
	script := ResizeDisk{
		Partitions: []ResizePartition{
			{
				Ref:  ResizePartitionRefLast,
				Size: ptrutils.PtrTo(DiskSize(1 * diskutils.GiB)),
			},
		},
	}
	err := script.IsValid()
	assert.NoError(t, err)
}

func TestResizeDiskIsValidMultiplePartitions(t *testing.T) {
	script := ResizeDisk{
		Partitions: []ResizePartition{
			{
				Ref:  ResizePartitionRefLast,
				Size: ptrutils.PtrTo(DiskSize(1 * diskutils.GiB)),
			},
			{
				Ref:  ResizePartitionRefLast,
				Size: ptrutils.PtrTo(DiskSize(2 * diskutils.GiB)),
			},
		},
	}
	err := script.IsValid()
	assert.ErrorContains(t, err, "resizing multiple partitions is not yet supported")
}
