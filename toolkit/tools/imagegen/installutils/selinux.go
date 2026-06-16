// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package installutils

import (
	"fmt"
	"os"
	"syscall"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
)

// Sets the SELinux context to use for any child processes started by the current OS thread.
// This is equivalent to using runcon, though with the advantage that it works with safechroot.
//
// Note: This function should only be run under the runtime.LockOSThread() lock.
func setSELinuxExecContext(context string) error {
	path := selinuxExecContextPath()

	logger.Log.Debugf("Set SELinux context [%s]", path)

	out, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = out.Write([]byte(context))
	if err != nil {
		return err
	}

	err = out.Close()
	if err != nil {
		return err
	}

	return nil
}

// Clears the explicit override of the SELinux context to use for child processes started by the current OS thread.
//
// Note: This function should only be run under the runtime.LockOSThread() lock and on the same thread that called
// setSELinuxExecContext().
func resetSELinuxExecContext() error {
	path := selinuxExecContextPath()

	logger.Log.Debugf("Clean SELinux context [%s]", path)

	out, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = out.Write([]byte(nil))
	if err != nil {
		return err
	}

	err = out.Close()
	if err != nil {
		return err
	}

	return nil
}

func selinuxExecContextPath() string {
	taskId := syscall.Gettid()
	path := fmt.Sprintf("/proc/self/task/%d/attr/exec", taskId)
	return path
}
