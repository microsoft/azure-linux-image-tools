// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/systemd"
)

type resolvConfType int

const (
	resolvConfTypeNone resolvConfType = iota
	resolvConfTypeSymlink
	resolvConfTypeFile
)

type resolvConfInfo struct {
	existingType resolvConfType
	fileContents string
	filePerms    os.FileMode
	symlinkPath  string
}

const (
	resolvConfPath = "/etc/resolv.conf"

	// This is the value that systemd-resolved sets as the /etc/resolv.conf symlink path.
	// It is unclear why systemd-resolved uses a relative path (../) instead of an absolute path (/).
	resolvSystemdStubPath = "../run/systemd/resolve/stub-resolv.conf"
)

// Override the resolv.conf file, so that in-chroot processes can access the network.
// For example, to install packages from packages.microsoft.com.
func overrideResolvConf(imageChroot *safechroot.Chroot) (resolvConfInfo, error) {
	logger.Log.Infof("Overriding resolv.conf file")

	imageResolveConfPath := filepath.Join(imageChroot.RootDir(), resolvConfPath)

	existing := resolvConfInfo{}

	stat, err := os.Lstat(imageResolveConfPath)
	if err != nil {
		logger.Log.Infof("Overriding resolv.conf file - 1")
		if !os.IsNotExist(err) {
			return resolvConfInfo{}, fmt.Errorf("failed to stat resolv.conf file:\n%w", err)
		}
		existing.existingType = resolvConfTypeNone
	} else if stat.Mode()&os.ModeSymlink != 0 {
		logger.Log.Infof("Overriding resolv.conf file - 2")
		symlinkPath, err := os.Readlink(imageResolveConfPath)
		if err != nil {
			return resolvConfInfo{}, fmt.Errorf("failed to read resolv.conf symlink's path:\n%w", err)
		}
		existing.existingType = resolvConfTypeSymlink
		existing.symlinkPath = symlinkPath
	} else {
		logger.Log.Infof("Overriding resolv.conf file - 3")
		fileContents, err := file.Read(imageResolveConfPath)
		if err != nil {
			return resolvConfInfo{}, fmt.Errorf("failed to read resolv.conf file:\n%w", err)
		}
		existing.existingType = resolvConfTypeFile
		existing.fileContents = fileContents
		existing.filePerms = stat.Mode().Perm()
	}
	logger.Log.Infof("Overriding resolv.conf file - 4")

	// Remove the existing resolv.conf file, if it exists.
	// Note: It is assumed that the image will have a process that runs on boot that will override the resolv.conf
	// file. For example, systemd-resolved. So, it isn't neccessary to make a back-up of the existing file.
	err = os.RemoveAll(imageResolveConfPath)
	if err != nil {
		return resolvConfInfo{}, fmt.Errorf("failed to delete existing resolv.conf file:\n%w", err)
	}

	logger.Log.Infof("Overriding resolv.conf file - 5 (%s) to (%s)", resolvConfPath, imageResolveConfPath)

	err = file.Copy(resolvConfPath, imageResolveConfPath)
	if err != nil {
		return resolvConfInfo{}, fmt.Errorf("failed to override resolv.conf file with host's resolv.conf:\n%w", err)
	}

	return existing, nil
}

func restoreResolvConf(existing resolvConfInfo, imageChroot *safechroot.Chroot) error {
	logger.Log.Infof("Restoring resolv.conf")

	imageResolveConfPath := filepath.Join(imageChroot.RootDir(), resolvConfPath)

	// Delete the overridden resolv.conf file.
	err := os.RemoveAll(imageResolveConfPath)
	if err != nil {
		return fmt.Errorf("failed to delete overridden resolv.conf file:\n%w", err)
	}

	switch existing.existingType {
	case resolvConfTypeNone:
		logger.Log.Infof("Restoring resolv.conf - 1")

		// Check if systemd-resolved is enabled.
		resolvedEnabled, err := systemd.IsServiceEnabled("systemd-resolved.service", imageChroot)
		if err != nil {
			return err
		}

		if resolvedEnabled {
			logger.Log.Infof("Restoring resolv.conf - 2 - %s, %s", resolvSystemdStubPath, imageResolveConfPath)
			logger.Log.Infof("Adding systemd-resolved resolv.conf symlink")

			// The systemd-resolved.service is enabled.
			// So, create the symlink for the resolv.conf file.
			// While technically this symlink will be automatically created on first boot, doing
			// it now is helpful for verity images where the root filesystem is readonly.
			err := os.Symlink(resolvSystemdStubPath, imageResolveConfPath)
			if err != nil {
				return fmt.Errorf("failed to create resolv.conf symlink:\n%w", err)
			}
		}
		logger.Log.Infof("Restoring resolv.conf - 3")

	case resolvConfTypeFile:
		logger.Log.Infof("Restoring resolv.conf - 4")
		logger.Log.Debugf("Restoring resolv.conf file")

		// Restore the resolv.conf file.
		err := file.WriteWithPerm(existing.fileContents, imageResolveConfPath, existing.filePerms)
		if err != nil {
			return fmt.Errorf("failed to restore resolv.conf file:\n%w", err)
		}

	case resolvConfTypeSymlink:
		logger.Log.Infof("Restoring resolv.conf - 5 - %s, %s", existing.symlinkPath, imageResolveConfPath)
		_, err := systemd.IsServiceEnabled("systemd-resolved.service", imageChroot)
		if err != nil {
			return err
		}
		logger.Log.Debugf("Restoring resolv.conf symlink")

		// Restore the resolv.conf symlink.
		err = os.Symlink(existing.symlinkPath, imageResolveConfPath)
		if err != nil {
			return fmt.Errorf("failed to restore resolv.conf file:\n%w", err)
		}

	default:
		return fmt.Errorf("unknown resolvConfType value (%v)", existing.existingType)
	}
	logger.Log.Infof("Restoring resolv.conf - 6")

	return nil
}
