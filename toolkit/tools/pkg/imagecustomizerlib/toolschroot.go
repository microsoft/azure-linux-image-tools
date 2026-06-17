// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
)

func initToolsChroot(ctx context.Context, toolsDir string) (*safechroot.Chroot, func(), error) {
	toolsChroot := safechroot.NewChroot(toolsDir, true)
	if err := toolsChroot.Initialize("", nil, nil, true); err != nil {
		return nil, nil, fmt.Errorf("failed to initialize tools chroot from %s:\n%w", toolsDir, err)
	}

	toolsResolvConf, err := overrideResolvConf(toolsChroot)
	if err != nil {
		if closeErr := toolsChroot.Close(false); closeErr != nil {
			logger.Log.Warnf("Failed to close tools chroot (%s) after resolv.conf override failure: %v",
				toolsDir, closeErr)
		}
		return nil, nil, fmt.Errorf("failed to override resolv.conf in tools chroot:\n%w", err)
	}

	cleanup := func() {
		if restoreErr := restoreResolvConf(ctx, toolsResolvConf, toolsChroot); restoreErr != nil {
			logger.Log.Warnf("Failed to restore resolv.conf in tools chroot (%s): %v",
				toolsDir, restoreErr)
		}
		if closeErr := toolsChroot.Close(false); closeErr != nil {
			logger.Log.Warnf("Failed to close tools chroot (%s): %v", toolsDir, closeErr)
		}
	}

	return toolsChroot, cleanup, nil
}
