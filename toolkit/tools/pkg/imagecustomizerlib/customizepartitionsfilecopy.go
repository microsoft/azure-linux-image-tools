// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/sirupsen/logrus"
)

var (
	// Partition copy errors
	ErrPartitionCopyTargetOsDetermination = NewImageCustomizerError("PartitionCopy:TargetOsDetermination", "failed to determine target OS of base image")
	ErrPartitionCopyFilesToNewLayout      = NewImageCustomizerError("PartitionCopy:FilesToNewLayout", "failed to copy files to new partition layout")
	ErrPartitionCopyFiles                 = NewImageCustomizerError("PartitionCopy:Files", "failed to copy partition files")
)

func customizePartitionsUsingFileCopy(ctx context.Context, buildDir string, storage imagecustomizerapi.Storage,
	osImageConnection *osImageConnection, newBuildImageFile string, distroHandler DistroHandler,
) (map[string]string, *osImageConnection, error) {
	ok := false

	imageMountPoint := filepath.Join(buildDir, "imageroot")

	_, err := osImageConnection.ConnectChroot(imageMountPoint, true /*readonly*/, false, /*readOnlyVerity*/
		distroHandler, buildDir, false /*includeDefaultMounts*/)
	if err != nil {
		return nil, nil, err
	}

	diskConfig := storage.Disks[0]

	installOSFunc := func(imageChroot *safechroot.Chroot) error {
		return copyFilesIntoNewDisk(osImageConnection.Chroot(), imageChroot)
	}

	partIdToPartUuid, newOsImageConnection, err := createNewImage(distroHandler, newBuildImageFile, diskConfig,
		storage.FileSystems, buildDir, "newimageroot", installOSFunc)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if !ok {
			newOsImageConnection.Close()
		}
	}()

	// Close the original image.
	err = osImageConnection.CleanClose()
	if err != nil {
		return nil, nil, err
	}

	return partIdToPartUuid, newOsImageConnection, nil
}

func copyFilesIntoNewDisk(existingImageChroot *safechroot.Chroot, newImageChroot *safechroot.Chroot) error {
	err := copyPartitionFiles(existingImageChroot.RootDir()+"/.", newImageChroot.RootDir())
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrPartitionCopyFilesToNewLayout, err)
	}
	return nil
}

func copyPartitionFiles(sourceRoot, targetRoot string) error {
	return copyPartitionFilesWithOptions(sourceRoot, targetRoot, true /*noClobber*/)
}

func copyPartitionFilesWithOptions(sourceRoot, targetRoot string, noClobber bool) error {
	// Notes:
	// `-a` ensures unix permissions, extended attributes (including SELinux), and sub-directories (-r) are copied.
	// `--no-dereference` ensures that symlinks are copied as symlinks.
	copyArgs := []string{
		"--verbose", "-a", "--no-dereference",
		sourceRoot, targetRoot,
	}

	if noClobber {
		copyArgs = append(copyArgs, "--no-clobber")
	}

	err := shell.NewExecBuilder("cp", copyArgs...).
		LogLevel(logrus.TraceLevel, logrus.DebugLevel).
		ErrorStderrLines(1).
		Execute()
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrPartitionCopyFiles, err)
	}

	return nil
}
