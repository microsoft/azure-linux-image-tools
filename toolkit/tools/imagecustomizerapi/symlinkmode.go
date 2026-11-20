// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import "fmt"

// SymlinkMode controls how additionalDirs and additionalFiles handle symbolic links.
type SymlinkMode string

const (
	// SymlinkModeUnspecified is the default when 'symlinkMode' is omitted. It behaves the
	// same as SymlinkModeDereference and does not require the 'symlink-mode' preview feature.
	SymlinkModeUnspecified SymlinkMode = ""

	// SymlinkModeDereference follows a symbolic link and copies its target's contents,
	// preserving the historical additionalDirs/additionalFiles behavior.
	SymlinkModeDereference SymlinkMode = "dereference"

	// SymlinkModePreserve recreates a symbolic link verbatim without reading its target on
	// the build host, supporting dangling links and links to special files.
	SymlinkModePreserve SymlinkMode = "preserve"
)

func (s SymlinkMode) IsValid() error {
	switch s {
	case SymlinkModeUnspecified, SymlinkModeDereference, SymlinkModePreserve:
		return nil
	default:
		return fmt.Errorf(
			"invalid symlink mode value (%s): must be one of ['', 'dereference', 'preserve']",
			s,
		)
	}
}
