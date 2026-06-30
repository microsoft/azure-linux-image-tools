// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
)

func CustomizeImageHelperCreate(ctx context.Context, rc *ResolvedConfig, toolsDir string,
	distroHandler DistroHandler,
) ([]fstabEntryPartNum, string, error) {
	logger.Log.Debugf("Customizing OS image")

	toolsChroot, err := initToolsChroot(ctx, toolsDir)
	if err != nil {
		return nil, "", err
	}
	defer toolsChroot.Close()

	imageMountPoint := filepath.Join(toolsDir, toolsRootImageDir)

	imageConnection, partitionsLayout, _, _, _, err := connectToExistingImage(ctx, rc.RawImageFile, rc.BuildDirAbs,
		imageMountPoint, true, false, false, false, distroHandler)
	if err != nil {
		return nil, "", err
	}
	defer imageConnection.Close()

	// Do the actual customizations.
	err = doOsCustomizations(ctx, rc, imageConnection, false /*partitionsCustomized*/, partitionsLayout, distroHandler,
		toolsChroot.Chroot(), true /*createImage*/)

	// Out of disk space errors can be difficult to diagnose.
	// So, warn about any partitions with low free space.
	warnOnLowFreeSpace(rc.BuildDirAbs, imageConnection)
	if err != nil {
		return nil, "", err
	}

	// Extract OS release info from rootfs for COSI
	osRelease, err := extractOSRelease(imageConnection)
	if err != nil {
		return nil, "", fmt.Errorf("failed to extract OS release from rootfs partition:\n%w", err)
	}

	err = imageConnection.CleanClose()
	if err != nil {
		return nil, "", err
	}

	err = toolsChroot.CleanClose()
	if err != nil {
		return nil, "", err
	}

	return partitionsLayout, osRelease, nil
}
