// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
)

// A wrapper around ImageConnection that keeps track of the partition layout of the OS image.
type osImageConnection struct {
	imageConnection  *imageconnection.ImageConnection
	diskPartitions   []diskutils.PartitionInfo
	partitionsLayout []fstabEntryPartNum
	verityMetadata   []verityDeviceMetadata
}

func (c *osImageConnection) DisconnectChroot() error {
	return c.imageConnection.DisconnectChroot()
}

func (c *osImageConnection) GetDiskPartitions() []diskutils.PartitionInfo {
	return c.diskPartitions
}

func (c *osImageConnection) RefreshDiskPartitions() ([]diskutils.PartitionInfo, error) {
	diskPartitions, err := diskutils.GetDiskPartitions(c.Loopback().DevicePath())
	if err != nil {
		return nil, err
	}

	c.diskPartitions = diskPartitions
	return diskPartitions, nil
}

func (c *osImageConnection) SetPartitionLayout(partitionsLayout []fstabEntryPartNum) error {
	if c.Chroot() != nil {
		return fmt.Errorf("internal error: cannot set partition layout when chroot is active")
	}

	c.partitionsLayout = partitionsLayout
	return nil
}

func (c *osImageConnection) ConnectChroot(rootMountPoint string, readonly bool, readOnlyVerity bool,
	distroHandler DistroHandler, buildDir string, includeDefaultMounts bool,
) ([]string, error) {
	if c.partitionsLayout == nil {
		return nil, fmt.Errorf("internal error: cannot connect chroot without partition layout being set")
	}

	if c.Chroot() != nil {
		return nil, fmt.Errorf("internal error: cannot connect chroot when chroot is already active")
	}

	mountPoints, readonlyPartUuids, tempDirectories, err := partitionLayoutToMountPoints(c.partitionsLayout,
		c.diskPartitions, readonly, readOnlyVerity, distroHandler, buildDir, includeDefaultMounts, rootMountPoint)
	if err != nil {
		return nil, fmt.Errorf("failed to find mount info for disk:\n%w", err)
	}

	c.imageConnection.AddOwnedDirectories(tempDirectories...)

	// Create chroot environment.
	err = c.imageConnection.ConnectChroot(rootMountPoint, false, nil, mountPoints, includeDefaultMounts)
	if err != nil {
		return nil, err
	}

	return readonlyPartUuids, nil
}

func (c *osImageConnection) ImageConnection() *imageconnection.ImageConnection {
	return c.imageConnection
}

func (c *osImageConnection) Chroot() *safechroot.Chroot {
	return c.imageConnection.Chroot()
}

func (c *osImageConnection) Loopback() *safeloopback.Loopback {
	return c.imageConnection.Loopback()
}

func (c *osImageConnection) Close() {
	c.imageConnection.Close()
}

func (c *osImageConnection) CleanClose() error {
	return c.imageConnection.CleanClose()
}
