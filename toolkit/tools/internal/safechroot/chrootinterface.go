// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package safechroot

type ChrootInterface interface {
	RootDir() string
	// ChrootDir returns a value that can be passed to the shell.ExecBuilder.Chroot function.
	ChrootDir() string
	AddFiles(filesToCopy ...FileToCopy) error
	// SecureJoin resolves a root-relative path within the root, clamping any symlink so the
	// result can never point outside the root. Use it instead of filepath.Join(RootDir(), path)
	// for any destination that is subsequently written.
	SecureJoin(path string) (string, error)
}
