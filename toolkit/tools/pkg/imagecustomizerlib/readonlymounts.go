// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"slices"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"golang.org/x/sys/unix"
)

func isPathOnReadOnlyMount(chrootPath string, imageChroot *safechroot.Chroot) bool {
	mounts := imageChroot.GetMountPoints()
	mountIndex, _, found := getMostSpecificPath(chrootPath, slices.All(mounts),
		func(mountPoint *safechroot.MountPoint) string {
			return mountPoint.GetTarget()
		})
	if !found {
		return false
	}

	mostSpecificMount := mounts[mountIndex]
	mountIsReadOnly := (mostSpecificMount.GetFlags() & unix.MS_RDONLY) != 0
	return mountIsReadOnly
}
