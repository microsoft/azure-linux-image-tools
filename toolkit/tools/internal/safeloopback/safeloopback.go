// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Package that assists with attach and detaching a loopback device cleanly.
package safeloopback

import (
	"fmt"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/kernelversion"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/version"
	"github.com/sirupsen/logrus"
)

const btrfsFileSystemType = "btrfs"

type Loopback struct {
	devicePath   string
	diskFilePath string
	diskIdMaj    string
	diskIdMin    string
	isAttached   bool
}

func NewLoopback(diskFilePath string) (*Loopback, error) {
	if !filepath.IsAbs(diskFilePath) {
		return nil, fmt.Errorf("loopback disk path must be absolute (%s)", diskFilePath)
	}

	loopback := &Loopback{
		diskFilePath: diskFilePath,
	}

	err := loopback.newLoopbackHelper()
	if err != nil {
		loopback.Close()
		return nil, err
	}

	return loopback, nil
}

func (l *Loopback) newLoopbackHelper() error {
	// Try to create the mount.
	devicePath, err := diskutils.SetupLoopbackDevice(l.diskFilePath)
	if err != nil {
		return err
	}

	l.devicePath = devicePath
	l.isAttached = true

	// Get the disk's IDs.
	maj, min, err := diskutils.GetDiskIds(l.devicePath)
	if err != nil {
		return err
	}

	l.diskIdMaj = maj
	l.diskIdMin = min

	// Ensure all the partitions have finished populating.
	err = diskutils.WaitForDiskDevice(devicePath)
	if err != nil {
		return err
	}

	return nil
}

func (l *Loopback) DevicePath() string {
	return l.devicePath
}

func (l *Loopback) DiskFilePath() string {
	return l.diskFilePath
}

// Request the loopback device refresh the size of the disk from file.
func (l *Loopback) RefreshDiskSize() error {
	return shell.NewExecBuilder("losetup", "--set-capacity", l.devicePath).
		ErrorStderrLines(1).
		Execute()
}

func (l *Loopback) Close() {
	err := l.close( /*async*/ true)
	if err != nil {
		logger.Log.Warnf("failed to close loopback: %s", err)
	}
}

func (l *Loopback) CleanClose() error {
	return l.close( /*async*/ false)
}

func (l *Loopback) close(async bool) error {
	if l.isAttached {
		forgetBtrfsFilesystems(l.devicePath)

		err := diskutils.DetachLoopbackDevice(l.devicePath)
		if err != nil {
			return err
		}

		l.isAttached = false
	}

	if !async {
		// The `losetup --detach` call happens asynchronously.
		// So, need to wait for it to complete.
		err := diskutils.WaitForLoopbackToDetach(l.devicePath, l.diskFilePath)
		if err != nil {
			return err
		}

		err = diskutils.BlockOnDiskIOByIds(l.devicePath, l.diskIdMaj, l.diskIdMin)
		if err != nil {
			return err
		}
	}

	return nil
}

// btrfsRegistrationFixedKernel is the first kernel that does not leave a stale btrfs registration
// behind. From this version, scanning a single-device btrfs neither registers it nor leaves an
// earlier registration in place.
var btrfsRegistrationFixedKernel = version.Version{6, 7}

// forgetBtrfsFilesystems unregisters the device's btrfs filesystems from the kernel before it is
// detached.
//
// The kernel keeps a global registry of scanned btrfs filesystems, keyed by filesystem UUID and
// holding the highest generation seen. udev populates it when a device appears, and before kernel
// 6.7 the entry is never reaped. So after customizing an image, which raises its generation, and
// detaching it, a pristine copy of the same source image is refused with EEXIST: "already registered
// with a higher generation". Retiring the entry here keeps the registry in step with the devices
// that produced it.
//
// Best-effort: failing to forget must not fail teardown.
func forgetBtrfsFilesystems(devicePath string) {
	kernelVersion, err := kernelversion.GetBuildHostKernelVersion()
	switch {
	case err != nil:
		// An unrecognized version is treated as affected, so that a parsing failure cannot quietly
		// disable this.
		logger.Log.Debugf("failed to read kernel version, forgetting btrfs filesystems anyway: %s", err)

	case kernelVersion.Ge(btrfsRegistrationFixedKernel):
		return
	}

	devices, err := btrfsDevices(devicePath)
	if err != nil {
		logger.Log.Debugf("failed to find btrfs filesystems on (%s): %s", devicePath, err)
		return
	}

	if len(devices) == 0 {
		return
	}

	args := append([]string{"device", "scan", "--forget"}, devices...)

	err = shell.NewExecBuilder("btrfs", args...).
		LogLevel(logrus.DebugLevel, logrus.DebugLevel).
		Execute()
	if err != nil {
		// The registration outlives the device, so leaving one behind makes a later customization of
		// the same base image fail to mount it.
		logger.Log.Warnf("Failed to unregister btrfs filesystems on (%s); customizing another image "+
			"from the same base may fail to mount it: %s", devicePath, err)
	}
}

// btrfsDevices returns the device's partitions that hold a btrfs filesystem, or the device itself
// when it holds one directly instead of a partition table.
func btrfsDevices(devicePath string) ([]string, error) {
	partitionTable, err := diskutils.ReadDiskPartitionTable(devicePath)
	if err != nil {
		return nil, err
	}

	if partitionTable == nil {
		// No partition table: the device may hold a filesystem directly.
		fileSystemType, err := diskutils.ReadFileSystemType(devicePath)
		if err != nil {
			return nil, err
		}

		if fileSystemType != btrfsFileSystemType {
			return nil, nil
		}

		return []string{devicePath}, nil
	}

	var devices []string
	for _, partition := range partitionTable.Partitions {
		if partition.FileSystemType == btrfsFileSystemType {
			devices = append(devices, partition.Path)
		}
	}

	return devices, nil
}
