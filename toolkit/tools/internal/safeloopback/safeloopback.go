// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Package that assists with attach and detaching a loopback device cleanly.
package safeloopback

import (
	"fmt"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/sirupsen/logrus"
)

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

// forgetBtrfsFilesystems unregisters the loopback device's btrfs filesystems from the kernel before
// the device is detached.
//
// The kernel keeps a global registry of scanned btrfs filesystems, keyed by filesystem UUID and
// holding the highest generation seen. udev populates it whenever a device appears. Before kernel
// 6.7 the entry is never reaped, so after customizing an image (which raises its generation) and
// detaching it, a pristine copy of the same source image is refused with EEXIST: "already registered
// with a higher generation". Retiring the entry here keeps the registry in step with the devices
// that produced it.
//
// Best-effort: the devices need not hold btrfs, and failing to forget must not fail teardown.
func forgetBtrfsFilesystems(devicePath string) {
	devices, err := filepath.Glob(devicePath + "p*")
	if err != nil {
		logger.Log.Debugf("failed to list partitions of (%s): %s", devicePath, err)
	}
	devices = append(devices, devicePath)

	args := append([]string{"device", "scan", "--forget"}, devices...)

	err = shell.NewExecBuilder("btrfs", args...).
		LogLevel(logrus.DebugLevel, logrus.DebugLevel).
		Execute()
	if err != nil {
		logger.Log.Debugf("failed to forget btrfs filesystems on (%s): %s", devicePath, err)
	}
}
