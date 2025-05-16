// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package file

import (
	"fmt"
	"io"
	"os"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
)

type FileCopyBuilder struct {
	Src            string
	Dst            string
	DirFileMode    os.FileMode
	ChangeFileMode bool
	FileMode       os.FileMode
	NoDereference  bool
}

func NewFileCopyBuilder(src string, dst string) FileCopyBuilder {
	return FileCopyBuilder{
		Src:            src,
		Dst:            dst,
		DirFileMode:    os.ModePerm,
		ChangeFileMode: false,
		FileMode:       os.ModePerm,
		NoDereference:  false,
	}
}

func (b FileCopyBuilder) SetDirFileMode(dirFileMode os.FileMode) FileCopyBuilder {
	b.DirFileMode = dirFileMode
	return b
}

func (b FileCopyBuilder) SetFileMode(fileMode os.FileMode) FileCopyBuilder {
	b.ChangeFileMode = true
	b.FileMode = fileMode
	return b
}

func (b FileCopyBuilder) SetNoDereference() FileCopyBuilder {
	b.NoDereference = true
	return b
}

func (b FileCopyBuilder) Run() (err error) {
	logger.Log.Debugf("Copying (%s) to (%s)", b.Src, b.Dst)

	if b.NoDereference && b.ChangeFileMode {
		return fmt.Errorf("cannot modify file permissions of symlinks")
	}

	if b.NoDereference {
		// Check if file is a symlink.
		srcFileInfo, err := os.Lstat(b.Src)
		if err != nil {
			return fmt.Errorf("failed to read source file link info:\n%w", err)
		}

		isSrcSymlink := srcFileInfo.Mode().Type() == os.ModeSymlink
		if isSrcSymlink {
			// Copy the symlink.
			symlinkPath, err := os.Readlink(b.Src)
			if err != nil {
				return fmt.Errorf("failed to read source symlink:\n%w", err)
			}

			err = os.Symlink(symlinkPath, b.Dst)
			if err != nil {
				return fmt.Errorf("failed to copy symlink:\n%w", err)
			}

			return nil
		}
	}

	srcFileInfo, err := os.Stat(b.Src)
	if err != nil {
		return fmt.Errorf("failed to read source file info:\n%w", err)
	}

	isSrcFile := !srcFileInfo.IsDir()
	if !isSrcFile {
		return fmt.Errorf("source (%s) is not a file", b.Src)
	}

	// Open source file.
	srcFile, err := os.Open(b.Src)
	if err != nil {
		return fmt.Errorf("failed to open source file:\n%w", err)
	}
	defer func() {
		if srcFile != nil {
			srcFile.Close()
		}
	}()

	dstFileMode := b.FileMode
	if !b.ChangeFileMode {
		// Copy the source file's permissions.
		dstFileMode = srcFileInfo.Mode()
	}

	err = CreateDestinationDir(b.Dst, b.DirFileMode)
	if err != nil {
		return fmt.Errorf("failed to create destination directory (%s):\n%w", b.Dst, err)
	}

	// Open/create destination file.
	dstFile, err := os.OpenFile(b.Dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, dstFileMode)
	if err != nil {
		return fmt.Errorf("failed to create destination file:\n%w", err)
	}
	defer func() {
		if dstFile != nil {
			dstFile.Close()
		}
	}()

	// The permissions given to OpenFile is subject to umask.
	// So, apply the permissions to ensure they match exactly.
	err = dstFile.Chmod(dstFileMode)
	if err != nil {
		return fmt.Errorf("failed to set destination file permissions:\n%w", err)
	}

	// Copy the file.
	// FYI: io.Copy uses the sendfile syscall where appropriate.
	// So, this should be pretty fast.
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file:\n%w", err)
	}

	// Close files.
	err = dstFile.Close()
	dstFile = nil
	if err != nil {
		return fmt.Errorf("failed to finalize destination file:\n%w", err)
	}

	err = srcFile.Close()
	srcFile = nil
	if err != nil {
		return fmt.Errorf("failed to finalize source file:\n%w", err)
	}

	return nil
}
