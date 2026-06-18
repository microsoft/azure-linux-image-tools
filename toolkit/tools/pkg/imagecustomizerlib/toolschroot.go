// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
)

// ToolsChroot wraps a safechroot used to run tdnf/dnf against a target image
// via --installroot. It owns the resolv.conf override on the tools chroot.
type ToolsChroot struct {
	chroot     *safechroot.Chroot
	resolvConf resolvConfInfo
	toolsDir   string
	closed     bool
}

func initToolsChroot(toolsDir string) (*ToolsChroot, error) {
	chroot := safechroot.NewChroot(toolsDir, true)
	if err := chroot.Initialize("", nil, nil, true); err != nil {
		return nil, fmt.Errorf("failed to initialize tools chroot from %s:\n%w", toolsDir, err)
	}

	resolvConf, err := overrideResolvConf(chroot)
	if err != nil {
		if closeErr := chroot.Close(false); closeErr != nil {
			logger.Log.Warnf("Failed to close tools chroot (%s) after resolv.conf override failure: %v",
				toolsDir, closeErr)
		}
		return nil, fmt.Errorf("failed to override resolv.conf in tools chroot:\n%w", err)
	}

	return &ToolsChroot{
		chroot:     chroot,
		resolvConf: resolvConf,
		toolsDir:   toolsDir,
	}, nil
}

// Chroot returns the underlying safechroot. Nil-safe.
func (t *ToolsChroot) Chroot() *safechroot.Chroot {
	if t == nil {
		return nil
	}
	return t.chroot
}

// CleanClose restores resolv.conf and closes the chroot, returning the first
// error. Use this in the success path. Safe to call multiple times.
func (t *ToolsChroot) CleanClose(ctx context.Context) error {
	if t == nil || t.closed {
		return nil
	}
	t.closed = true

	if err := restoreResolvConf(ctx, t.resolvConf, t.chroot); err != nil {
		// Still attempt to close so the chroot mount is not leaked.
		if closeErr := t.chroot.Close(false); closeErr != nil {
			logger.Log.Warnf("Failed to close tools chroot (%s) after restoreResolvConf failure: %v",
				t.toolsDir, closeErr)
		}
		return fmt.Errorf("failed to restore resolv.conf in tools chroot:\n%w", err)
	}
	return t.chroot.Close(false)
}

// Close is a best-effort cleanup intended for `defer`. Errors are logged. Safe
// to call multiple times.
func (t *ToolsChroot) Close(ctx context.Context) {
	if t == nil || t.closed {
		return
	}
	t.closed = true

	if err := restoreResolvConf(ctx, t.resolvConf, t.chroot); err != nil {
		logger.Log.Warnf("Failed to restore resolv.conf in tools chroot (%s): %v", t.toolsDir, err)
	}
	if err := t.chroot.Close(false); err != nil {
		logger.Log.Warnf("Failed to close tools chroot (%s): %v", t.toolsDir, err)
	}
}
