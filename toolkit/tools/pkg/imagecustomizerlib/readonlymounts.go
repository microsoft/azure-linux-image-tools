// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"path/filepath"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"golang.org/x/sys/unix"
)

func isPathOnReadOnlyMount(chrootPath string, imageChroot *safechroot.Chroot) bool {
	mostSpecificMount := (*safechroot.MountPoint)(nil)

	mounts := imageChroot.GetMountPoints()
	for _, mount := range mounts {
		relativePath, err := filepath.Rel(mount.GetTarget(), chrootPath)
		if err != nil || strings.HasPrefix(relativePath, "../") {
			// Path is not relative to the mount.
			continue
		}

		if mostSpecificMount == nil || len(mount.GetTarget()) > len(mostSpecificMount.GetTarget()) {
			mostSpecificMount = mount
		}
	}

	if mostSpecificMount == nil {
		return false
	}

	mountIsReadOnly := (mostSpecificMount.GetFlags() & unix.MS_RDONLY) != 0
	return mountIsReadOnly
}
