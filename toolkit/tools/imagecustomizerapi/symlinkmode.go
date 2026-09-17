// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import "fmt"

// SymlinkMode controls how additionalDirs handles symbolic links.
type SymlinkMode string

const (
	SymlinkModeUnspecified SymlinkMode = ""
	SymlinkModeDereference SymlinkMode = "dereference"
	SymlinkModePreserve    SymlinkMode = "preserve"
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
