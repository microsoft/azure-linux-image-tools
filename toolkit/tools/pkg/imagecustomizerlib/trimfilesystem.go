// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"golang.org/x/sys/unix"
)

var (
	ErrFilesystemTrim = NewImageCustomizerError("Filesystem:Trim", "failed to trim filesystem")
)

func trimFileSystems(imageConnection *imageconnection.ImageConnection) error {
	logger.Log.Infof("Trimming filesystems")

	for _, mountPoint := range getNonSpecialChrootMountPoints(imageConnection.Chroot()) {
		if (mountPoint.GetFlags() & unix.MS_RDONLY) != 0 {
			// Skip read-only filesystems.
			continue
		}

		fullMountPoint := filepath.Join(imageConnection.Chroot().RootDir(), mountPoint.GetTarget())

		logger.Log.Debugf("Trimming filesystem (mount='%s', fstype='%s')", mountPoint.GetTarget(),
			mountPoint.GetFSType())

		err := trimFileSystemIfSupported(fullMountPoint)
		if err != nil {
			return err
		}
	}

	return nil
}

func trimFileSystemIfSupported(mountPoint string) error {
	err := trimFileSystemIfSupportedHelper(mountPoint)
	if err != nil {
		return fmt.Errorf("%w (path='%s')", ErrFilesystemTrim, mountPoint)
	}

	return nil
}

// Calls the FITRIM (i.e. fstrim) IOCTL on a mounted filesystem, if it is supported by the filesystem.
func trimFileSystemIfSupportedHelper(mountPoint string) error {
	const fitrimIoctl = uintptr(0xc0185879)

	type fstrimRange struct {
		start  uint64
		len    uint64
		minlen uint64
	}

	trimRange := fstrimRange{
		len: math.MaxUint64,
	}

	file, err := os.Open(mountPoint)
	if err != nil {
		return err
	}
	defer file.Close()

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), fitrimIoctl, uintptr(unsafe.Pointer(&trimRange)))
	if errno == syscall.EOPNOTSUPP {
		logger.Log.Debugf("Trimming filesystem not supported (mount='%s')", mountPoint)
	} else if errno != 0 {
		return errno
	}

	return nil
}
