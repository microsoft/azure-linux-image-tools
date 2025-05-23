// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Package that assists with mounting and unmounting cleanly.
package safemount

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/retry"
	"golang.org/x/sys/unix"
)

type Mount struct {
	target           string
	isMounted        bool
	fileOrDirCreated bool
}

// Creates a new system mount.
func NewMount(source, target, fstype string, flags uintptr, data string, makeAndDeleteDir bool) (*Mount, error) {
	var err error

	mount := &Mount{
		target: target,
	}

	// Try to create the mount.
	err = mount.newMountHelper(source, target, fstype, flags, data, makeAndDeleteDir)
	if err != nil {
		// Cleanup anything created during the failed mount.
		mount.Close()
		return nil, err
	}

	return mount, nil
}

func (m *Mount) newMountHelper(source, target, fstype string, flags uintptr, data string, makeAndDeleteDir bool) error {
	var err error

	logger.Log.Debugf("Mounting: source: (%s), target: (%s), fstype: (%s), flags: (%#x), data: (%s)",
		source, target, fstype, flags, data)

	if makeAndDeleteDir {
		sourceInfo, err := os.Stat(source)
		if err != nil {
			return fmt.Errorf("failed to stat mount source (%s):\n%w", source, err)
		}

		sourceIsFile := (sourceInfo.Mode() & os.ModeType) == 0
		isBindMount := (flags & unix.MS_BIND) == unix.MS_BIND
		if !sourceIsFile || !isBindMount {
			// Create the mount target directory.
			err = os.MkdirAll(target, os.ModePerm)
			if err != nil {
				return fmt.Errorf("failed to create mount directory (%s):\n%w", target, err)
			}
		} else {
			// Mount is a file bind mount.
			// So, create a placeholder file instead of a placeholder directory.

			// Create parent directory.
			err = os.MkdirAll(filepath.Dir(target), os.ModePerm)
			if err != nil {
				return fmt.Errorf("failed to create mount target parent directory (%s):\n%w", target, err)
			}

			// Create placeholder file.
			err = os.WriteFile(target, []byte(nil), os.ModePerm)
			if err != nil {
				return fmt.Errorf("failed to create mount target file (%s):\n%w", target, err)
			}
		}

		m.fileOrDirCreated = true
	}

	// Create the mount.
	err = unix.Mount(source, target, fstype, flags, data)
	if err != nil {
		return fmt.Errorf("failed to mount (%s) to (%s):\n%w", source, target, err)
	}

	m.isMounted = true
	return nil
}

// Target returns the target directory of the mount.
func (m *Mount) Target() string {
	return m.target
}

// Close removes the system mount and fails if the device is still busy.
// CleanClose and Close are safe to call multiple times.
func (m *Mount) CleanClose() error {
	return m.close(false /*async*/)
}

// Close removes the system mount.
// The unmount is performed asynchronously. This reduces the likelihood of the unmount failing
// (thus ensuring the user's system is left in a clean state). But it doesn't provide any
// guarantees about the validity of the written bits.
// CleanClose and Close are safe to call multiple times.
func (m *Mount) Close() {
	err := m.close(true /*async*/)
	if err != nil {
		logger.Log.Warnf("%s", err)
	}
}

func (m *Mount) close(async bool) error {
	var err error

	if m.isMounted {
		if !async {
			logger.Log.Debugf("Unmounting (%s)", m.target)
			_, err = retry.RunWithExpBackoff(context.Background(),
				func() error {
					logger.Log.Debugf("Trying to unmount (%s)", m.target)
					umountErr := unix.Unmount(m.target, 0)
					return umountErr
				},
				3, time.Second, 2.0)
			if err != nil {
				return fmt.Errorf("failed to unmount (%s):\n%w", m.target, err)
			}
		} else {
			logger.Log.Debugf("Asynchronously unmounting (%s)", m.target)
			err = unix.Unmount(m.target, unix.MNT_DETACH)
			if err != nil {
				return fmt.Errorf("failed to asynchronously unmount (%s) (please manually unmount device):\n%w", m.target, err)
			}
		}

		m.isMounted = false
	}

	if m.fileOrDirCreated {
		logger.Log.Debugf("Deleting directory (%s)", m.target)

		// Note: Do not use `RemoveAll` here in case the unmount silently failed.
		// (This is unlikely. But "belt and braces".)
		err = os.Remove(m.target)
		if err != nil {
			return fmt.Errorf("failed to delete mount directory (%s):\n%w", m.target, err)
		}

		m.fileOrDirCreated = false
	}

	return nil
}
