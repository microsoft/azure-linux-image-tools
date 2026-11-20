// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imageconnection

import (
	"fmt"
	"os"
	"slices"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
)

type ImageConnection struct {
	loopback         *safeloopback.Loopback
	chroot           *safechroot.Chroot
	ownedDirectories []string
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

	return nil
}

func (c *ImageConnection) DisconnectChroot() error {
	if c.chroot != nil {
		err := c.chroot.Close()
		if err != nil {
			return err
		}

		c.chroot = nil
	}

	for len(c.ownedDirectories) > 0 {
		dir := c.ownedDirectories[len(c.ownedDirectories)-1]
		err := os.RemoveAll(dir)
		if err != nil {
			return fmt.Errorf("failed to remove image connection directory (%s):\n%w", dir, err)
		}

		c.ownedDirectories = c.ownedDirectories[:len(c.ownedDirectories)-1]
	}

	return nil
}

func (c *ImageConnection) AddOwnedDirectories(dirs ...string) {
	c.ownedDirectories = append(c.ownedDirectories, dirs...)
}

func (c *ImageConnection) Chroot() *safechroot.Chroot {
	return c.chroot
}

func (c *ImageConnection) Loopback() *safeloopback.Loopback {
	return c.loopback
}

func (c *ImageConnection) Close() {
	if c.chroot != nil {
		c.chroot.Close()
		c.chroot = nil
	}

	for _, dir := range slices.Backward(c.ownedDirectories) {
		os.RemoveAll(dir)
	}

	c.ownedDirectories = nil

	if c.loopback != nil {
		c.loopback.Close()
		c.loopback = nil
	}
}

func (c *ImageConnection) CleanClose() error {
	err := c.DisconnectChroot()
	if err != nil {
		return err
	}

	err = c.loopback.CleanClose()
	if err != nil {
		return err
	}

	c.loopback = nil

	return nil
}
