// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/envfile"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/sliceutils"
	"golang.org/x/sys/unix"
)

// Flatcar doesn't do something as convenient as declaring all their partitions in an /etc/fstab file that is easy to
// find. Instead, its partition mounts are spread out between systemd *.mount units, kernel cmdline args, and custom
// dracut modules. It isn't practical or sufficiently generic to try to piece together all these disparate sources. So,
// instead just detect that the OS image is flatcar and then provide hardcoded fstab entries.
func findFstabEntriesForFlatcar(diskPartitions []diskutils.PartitionInfo, buildDir string,
) ([]diskutils.FstabEntry, bool, error) {
	isFlatcar, err := isFlatcarOsImage(diskPartitions, buildDir)
	if err != nil {
		return nil, false, err
	}
	if !isFlatcar {
		return nil, false, nil
	}

	// TODO
}

func isFlatcarOsImage(diskPartitions []diskutils.PartitionInfo, buildDir string) (bool, error) {
	usrPartTypeUuid := imagecustomizerapi.PartitionTypeToUuid[imagecustomizerapi.PartitionTypeUsr]

	// Check if there are any partitions that declare themselves to be a usr partition.
	usrPartitions := sliceutils.FindMatches(diskPartitions, func(partition diskutils.PartitionInfo) bool {
		return partition.PartitionTypeUuid == usrPartTypeUuid && partition.FileSystemType != ""
	})
	if len(usrPartitions) != 1 {
		return false, nil
	}

	usrPartition := usrPartitions[0]

	osReleaseStr, found, err := readUsrOsReleaseFile(usrPartition, buildDir)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}

	fields, err := envfile.ParseEnv(osReleaseStr)
	if err != nil {
		return false, fmt.Errorf("failed to parse usr partition's /lib/os-release file:\n%w", err)
	}

	idValue, idFound := fields["ID"]
	idLikeValue, idLikeFound := fields["ID_LIKE"]

	isFlatcar := (idFound && idValue == "flatcar") || (idLikeFound && idLikeValue == "flatcar")
	return isFlatcar, nil
}

func readUsrOsReleaseFile(usrPartition diskutils.PartitionInfo, buildDir string) (string, bool, error) {
	mountDir := filepath.Join(buildDir, tmpPartitionDirName)
	partitionMount, err := safemount.NewMount(usrPartition.Path, mountDir, usrPartition.FileSystemType,
		unix.MS_RDONLY, "", true)
	if err != nil {
		return "", false, fmt.Errorf("failed to mount partition (%s):\n%w", usrPartition.Path, err)
	}
	defer partitionMount.Close()

	osReleasePath := filepath.Join(mountDir, "lib/os-release")
	osReleaseStr, err := file.Read(osReleasePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", false, err
		}

		err = partitionMount.CleanClose()
		if err != nil {
			return "", false, fmt.Errorf("failed to close partition mount (%s):\n%w", usrPartition.Path, err)
		}

		return "", false, nil
	}

	err = partitionMount.CleanClose()
	if err != nil {
		return "", false, fmt.Errorf("failed to close partition mount (%s):\n%w", usrPartition.Path, err)
	}

	return osReleaseStr, true, nil
}
