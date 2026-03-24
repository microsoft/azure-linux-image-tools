// Code copied and modified from the Go project's lp_unix.go file.
// The original copyright is as follows:
//
//   Copyright 2010 The Go Authors. All rights reserved.
//   Use of this source code is governed by a BSD-style
//   license that can be found in the LICENSE file.
//
// Modifications have copyright as follows:
//
//   Copyright (c) Microsoft Corporation.
//   Licensed under the MIT License.

package shell

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

// Checks if a file is executable.
func findExecutable(file string) error {
	d, err := os.Stat(file)
	if err != nil {
		return err
	}
	m := d.Mode()
	if m.IsDir() {
		return syscall.EISDIR
	}
	err = syscall.Faccessat(unix.AT_FDCWD, file, unix.X_OK, unix.AT_EACCESS)
	// ENOSYS means Eaccess is not available or not implemented.
	// EPERM can be returned by Linux containers employing seccomp.
	// In both cases, fall back to checking the permission bits.
	if err == nil || (err != syscall.ENOSYS && err != syscall.EPERM) {
		return err
	}
	if m&0111 != 0 {
		return nil
	}
	return fs.ErrPermission
}

// chrootLookPath searches for an executable file named 'file' in 'dirs' within the 'rootDir'.
// Returns a path relative to 'rootDir'.
func chrootLookPath(file string, rootDir string, dirs []string) (string, error) {
	for _, dir := range dirs {
		path := filepath.Join(rootDir, dir, file)
		if err := findExecutable(path); err == nil {
			return filepath.Join(dir, file), nil
		}
	}
	return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
}
