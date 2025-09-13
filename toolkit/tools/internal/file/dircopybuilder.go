// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package file

import (
	"io/fs"
)

type FileCopyUpdateMode int

const (
	// Overwrite any existing file.
	FileCopyUpdateModeOverwriteAll FileCopyUpdateMode = iota
	// Fail if there is a conflicting existing file.
	FileCopyUpdateModeFailExisting
	// Skip (leave alone) any conflicting existing files.
	FileCopyUpdateModeSkipExisting
)

type DirCopyBuilder struct {
	// Source directory
	Src string
	// Destination directory
	Dst string
	// How existing files should be handled.
	UpdateMode FileCopyUpdateMode
	//
	NewDirPermissions    *fs.FileMode
	ChildFilePermissions *fs.FileMode
	MergedDirPermissions *fs.FileMode
}

func NewDirCopyBuilder(src string, dst string) DirCopyBuilder {
	return DirCopyBuilder{
		Src: src,
		Dst: dst,
	}
}
