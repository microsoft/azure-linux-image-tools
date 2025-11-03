// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"go.opentelemetry.io/otel"
)

var (
	ErrSelinuxPolicyImageConnection = NewImageCustomizerError("SelinuxPolicy:ImageConnection", "failed to connect to image for SELinux policy extraction")
	ErrSelinuxPolicyDirNotFound     = NewImageCustomizerError("SelinuxPolicy:DirNotFound", "SELinux policy directory not found in image")
	ErrSelinuxPolicyDirCopy         = NewImageCustomizerError("SelinuxPolicy:DirCopy", "failed to copy SELinux policy directory")
	ErrSelinuxPolicyOutputDirCreate = NewImageCustomizerError("SelinuxPolicy:OutputDirCreate", "failed to create output directory for SELinux policy")
	ErrSelinuxPolicyMountPointFind  = NewImageCustomizerError("SelinuxPolicy:MountPointFind", "failed to find mount point for SELinux policy directory")
)

const (
	selinuxPolicyPath = "/etc/selinux/targeted"
)

func outputSelinuxPolicy(ctx context.Context, outputDir string, buildDir string, buildImage string) error {
	logger.Log.Infof("Extracting SELinux policy from image")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "output_selinux_policy")
	defer span.End()

	err := os.MkdirAll(outputDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("%w (path='%s'):\n%w", ErrSelinuxPolicyOutputDirCreate, outputDir, err)
	}

	// Connect to the image with read-only mounts.
	// Use connectToExistingImage which automatically mounts all partitions based on fstab.
	imageConnection, _, _, _, err := connectToExistingImage(ctx, buildImage, buildDir, "selinux-extract",
		true /*includeDefaultMounts*/, true /*readonly*/, true /*readOnlyVerity*/, false /*ignoreOverlays*/)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrSelinuxPolicyImageConnection, err)
	}
	defer imageConnection.Close()

	imageChroot := imageConnection.Chroot()
	chrootDir := imageChroot.RootDir()

	selinuxPolicyFullPath := filepath.Join(chrootDir, selinuxPolicyPath)

	_, err = os.Stat(selinuxPolicyFullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w (path='%s'):\nthe SELinux policy directory does not exist in the image",
				ErrSelinuxPolicyDirNotFound, selinuxPolicyPath)
		}
		return fmt.Errorf("%w (path='%s'):\n%w", ErrSelinuxPolicyDirNotFound, selinuxPolicyPath, err)
	}

	destPath := filepath.Join(outputDir, "targeted")
	logger.Log.Infof("Copying SELinux policy to %s", destPath)

	err = file.CopyDir(selinuxPolicyFullPath, destPath, 0755, 0644, nil)
	if err != nil {
		return fmt.Errorf("%w (src='%s', dest='%s'):\n%w",
			ErrSelinuxPolicyDirCopy, selinuxPolicyFullPath, destPath, err)
	}

	err = imageConnection.CleanClose()
	if err != nil {
		return fmt.Errorf("failed to cleanly close image connection:\n%w", err)
	}

	logger.Log.Infof("Successfully extracted SELinux policy to %s", outputDir)

	return nil
}
