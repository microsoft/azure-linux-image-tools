// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/targetos"
	"github.com/sirupsen/logrus"
)

var (
	// Partition copy errors
	ErrPartitionCopyTargetOsDetermination = NewImageCustomizerError("PartitionCopy:TargetOsDetermination", "failed to determine target OS of base image")
	ErrPartitionCopyFilesToNewLayout      = NewImageCustomizerError("PartitionCopy:FilesToNewLayout", "failed to copy files to new partition layout")
	ErrPartitionCopyFiles                 = NewImageCustomizerError("PartitionCopy:Files", "failed to copy partition files")
)

func customizePartitionsUsingFileCopy(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.Config,
	buildImageFile string, newBuildImageFile string,
) (map[string]string, error) {
	existingImageConnection, _, _, _, err := connectToExistingImage(ctx, buildImageFile, buildDir, "imageroot", false,
		true, false, false)
	if err != nil {
		return nil, err
	}
	defer existingImageConnection.Close()

	targetOs, err := targetos.GetInstalledTargetOs(existingImageConnection.Chroot().RootDir())
	if err != nil {
		return nil, fmt.Errorf("%w:\n%w", ErrPartitionCopyTargetOsDetermination, err)
	}

	diskConfig := config.Storage.Disks[0]

	installOSFunc := func(imageChroot *safechroot.Chroot) error {
		return copyFilesIntoNewDisk(existingImageConnection.Chroot(), imageChroot)
	}

	partIdToPartUuid, err := CreateNewImage(targetOs, newBuildImageFile, diskConfig, config.Storage.FileSystems,
		buildDir, "newimageroot", installOSFunc)
	if err != nil {
		return nil, err
	}

	err = existingImageConnection.CleanClose()
	if err != nil {
		return nil, err
	}

	return partIdToPartUuid, nil
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
		"--verbose", "-a", "--no-dereference", "--sparse", "always",
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
