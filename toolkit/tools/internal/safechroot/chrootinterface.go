// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package safechroot

type ChrootInterface interface {
	RootDir() string
	// ChrootDir returns a value that can be passed to the shell.ExecBuilder.Chroot function.
	ChrootDir() string
	AddFiles(filesToCopy ...FileToCopy) error
}
