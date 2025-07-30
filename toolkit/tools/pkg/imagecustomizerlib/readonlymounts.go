// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"path/filepath"
	"slices"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
)

func isPathOnReadOnlyMount(chrootPath string, imageChroot *safechroot.Chroot, readOnlyMounts []string,
) bool {
	mostSpecificMountTarget := ""

	mounts := imageChroot.GetMountPoints()
	for _, mount := range mounts {
		mountTarget := mount.GetTarget()

		_, err := filepath.Rel(mountTarget, chrootPath)
		if err != nil {
			// Path is not relative to the mount.
			continue
		}

		if len(mountTarget) > len(mostSpecificMountTarget) {
			mostSpecificMountTarget = mountTarget
		}
	}

	if mostSpecificMountTarget == "" {
		return false
	}

	mountIsReadOnly := slices.Contains(readOnlyMounts, mostSpecificMountTarget)
	return mountIsReadOnly
}
