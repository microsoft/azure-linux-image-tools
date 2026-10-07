// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
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
	buildImageFile string, newBuildImageFile string, distroHandler DistroHandler,
) (map[string]string, error) {
	imageMountPoint := filepath.Join(buildDir, "imageroot")

	existingImageConnection, _, _, _, _, err := connectToExistingImage(ctx, buildImageFile, buildDir, imageMountPoint,
		false, true, false, false, distroHandler)
	if err != nil {
		return nil, err
	}
	defer existingImageConnection.Close()

	diskConfig := storage.Disks[0]

	installOSFunc := func(imageChroot *safechroot.Chroot) error {
		err := copyFilesIntoNewDisk(existingImageConnection.Chroot(), imageChroot)
		if err != nil {
			return err
		}
		if aclHandler, ok := distroHandler.(*aclDistroHandler); ok && aclHandler.rootVerityLayout {
			return migrateAclEtcToRoot(imageChroot.RootDir())
		}
		return nil
	}

	partIdToPartUuid, err := CreateNewImage(distroHandler, newBuildImageFile, diskConfig, storage.FileSystems,
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

func migrateAclEtcToRoot(rootDir string) error {
	baseline := filepath.Join(rootDir, aclEtcBaselineDir)
	etc := filepath.Join(rootDir, "etc")
	if _, err := os.Stat(baseline); err != nil {
		return fmt.Errorf("ACL /etc baseline missing (%s): %w", baseline, err)
	}
	if err := copyPartitionFilesWithOptions(baseline+"/.", etc, true); err != nil {
		return fmt.Errorf("failed to migrate ACL /etc into root filesystem: %w", err)
	}
	if err := os.RemoveAll(baseline); err != nil {
		return fmt.Errorf("failed to remove old ACL /etc baseline: %w", err)
	}
	return nil
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
