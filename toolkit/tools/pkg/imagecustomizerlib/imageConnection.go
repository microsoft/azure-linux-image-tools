// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safeloopback"
)

const (
	testImageRootDirName = "testimageroot"
)

type ImageConnection struct {
	loopback            *safeloopback.Loopback
	chroot              *safechroot.Chroot
	chrootIsExistingDir bool
}
type MountPoint struct {
	PartitionNum   int
	Path           string
	FileSystemType string
	Flags          uintptr
}

func NewImageConnection() *ImageConnection {
	return &ImageConnection{}
}

func (c *ImageConnection) ConnectLoopback(diskFilePath string) error {
	if c.loopback != nil {
		return fmt.Errorf("loopback already connected")
	}

	loopback, err := safeloopback.NewLoopback(diskFilePath)
	if err != nil {
		return fmt.Errorf("failed to mount raw disk (%s) as a loopback device:\n%w", diskFilePath, err)
	}
	c.loopback = loopback
	return nil
}

func (c *ImageConnection) ConnectChroot(rootDir string, isExistingDir bool, extraDirectories []string,
	extraMountPoints []*safechroot.MountPoint, includeDefaultMounts bool,
) error {
	if c.chroot != nil {
		return fmt.Errorf("chroot already connected")
	}

	chroot := safechroot.NewChroot(rootDir, isExistingDir)
	err := chroot.Initialize("", extraDirectories, extraMountPoints, includeDefaultMounts)
	if err != nil {
		return err
	}
	c.chroot = chroot
	c.chrootIsExistingDir = isExistingDir

	return nil
}

func (c *ImageConnection) Chroot() *safechroot.Chroot {
	return c.chroot
}

func (c *ImageConnection) Loopback() *safeloopback.Loopback {
	return c.loopback
}

func (c *ImageConnection) Close() {
	if c.chroot != nil {
		c.chroot.Close(c.chrootIsExistingDir)
	}

	if c.loopback != nil {
		c.loopback.Close()
	}
}

func (c *ImageConnection) CleanClose() error {
	err := c.chroot.Close(c.chrootIsExistingDir)
	if err != nil {
		return err
	}

	err = c.loopback.CleanClose()
	if err != nil {
		return err
	}

	return nil
}

func ConnectToImage(buildDir string, imageFilePath string, includeDefaultMounts bool, mounts []MountPoint,
) (*ImageConnection, error) {
	imageConnection := NewImageConnection()
	err := imageConnection.ConnectLoopback(imageFilePath)
	if err != nil {
		imageConnection.Close()
		return nil, err
	}

	rootDir := filepath.Join(buildDir, testImageRootDirName)

	mountPoints := []*safechroot.MountPoint(nil)
	for _, mount := range mounts {
		devPath := partitionDevPath(imageConnection, mount.PartitionNum)

		var mountPoint *safechroot.MountPoint
		if mount.Path == "/" {
			mountPoint = safechroot.NewPreDefaultsMountPoint(devPath, mount.Path, mount.FileSystemType, mount.Flags,
				"")
		} else {
			mountPoint = safechroot.NewMountPoint(devPath, mount.Path, mount.FileSystemType, mount.Flags, "")
		}

		mountPoints = append(mountPoints, mountPoint)
	}

	err = imageConnection.ConnectChroot(rootDir, false, []string{}, mountPoints, includeDefaultMounts)
	if err != nil {
		imageConnection.Close()
		return nil, err
	}

	return imageConnection, nil
}

func partitionDevPath(imageConnection *ImageConnection, partitionNum int) string {
	devPath := fmt.Sprintf("%sp%d", imageConnection.Loopback().DevicePath(), partitionNum)
	return devPath
}
