// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/envfile"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"go.opentelemetry.io/otel"
)

var (
	ErrSelinuxPolicyImageConnection = NewImageCustomizerError("SelinuxPolicy:ImageConnection", "failed to connect to image for SELinux policy extraction")
	ErrSelinuxPolicyDirNotFound     = NewImageCustomizerError("SelinuxPolicy:DirNotFound", "SELinux policy directory cannot be read")
	ErrSelinuxPolicyDirCopy         = NewImageCustomizerError("SelinuxPolicy:DirCopy", "failed to copy SELinux policy directory")
	ErrSelinuxPolicyOutputDirCreate = NewImageCustomizerError("SelinuxPolicy:OutputDirCreate", "failed to create output directory for SELinux policy")
	ErrSelinuxPolicyMountPointFind  = NewImageCustomizerError("SelinuxPolicy:MountPointFind", "failed to find mount point for SELinux policy directory")
	ErrSelinuxPolicyConfigNotFound  = NewImageCustomizerError("SelinuxPolicy:ConfigNotFound", "SELinux config file not found in image")
	ErrSelinuxPolicyTypeNotFound    = NewImageCustomizerError("SelinuxPolicy:TypeNotFound", "SELINUXTYPE not found in SELinux config file")
)

const (
	selinuxConfigPath = "/etc/selinux/config"
	selinuxBaseDir    = "/etc/selinux"
)

func readSelinuxType(chrootDir string) (string, error) {
	configPath := filepath.Join(chrootDir, selinuxConfigPath)

	fields, err := envfile.ParseEnvFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w (path='%s')", ErrSelinuxPolicyConfigNotFound, selinuxConfigPath)
		}
		return "", fmt.Errorf("failed to read SELinux config file (path='%s'):\n%w", selinuxConfigPath, err)
	}

	selinuxType, found := fields["SELINUXTYPE"]
	if !found || selinuxType == "" {
		return "", fmt.Errorf("%w (path='%s')", ErrSelinuxPolicyTypeNotFound, selinuxConfigPath)
	}

	return selinuxType, nil
}

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
		false /*includeDefaultMounts*/, true /*readonly*/, true /*readOnlyVerity*/, true /*ignoreOverlays*/)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrSelinuxPolicyImageConnection, err)
	}
	defer imageConnection.Close()

	imageChroot := imageConnection.Chroot()
	chrootDir := imageChroot.RootDir()

	// Read SELINUXTYPE from /etc/selinux/config
	selinuxType, err := readSelinuxType(chrootDir)
	if err != nil {
		return err
	}

	logger.Log.Infof("Found SELINUXTYPE=%s in SELinux config", selinuxType)

	// Build the policy path using the dynamic SELINUXTYPE
	selinuxPolicyPath := filepath.Join(selinuxBaseDir, selinuxType)
	selinuxPolicyFullPath := filepath.Join(chrootDir, selinuxPolicyPath)

	_, err = os.Stat(selinuxPolicyFullPath)
	if err != nil {
		return fmt.Errorf("%w (path='%s'):\n%w", ErrSelinuxPolicyDirNotFound, selinuxPolicyPath, err)
	}

	destPath := filepath.Join(outputDir, selinuxType)
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
