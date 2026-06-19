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
	ctx        context.Context
	chroot     *safechroot.Chroot
	resolvConf *resolvConfInfo
	toolsDir   string
}

func initToolsChroot(ctx context.Context, toolsDir string) (*ToolsChroot, error) {
	t := &ToolsChroot{
		ctx:      ctx,
		toolsDir: toolsDir,
	}

	err := t.initToolsChroot()
	if err != nil {
		t.Close()
		return nil, err
	}

	return t, nil
}

func (t *ToolsChroot) initToolsChroot() error {
	t.chroot = safechroot.NewChroot(t.toolsDir, true)
	if err := t.chroot.Initialize("", nil, nil, true); err != nil {
		return fmt.Errorf("failed to initialize tools chroot from %s:\n%w", t.toolsDir, err)
	}

	resolvConf, err := overrideResolvConf(t.chroot)
	if err != nil {
		return fmt.Errorf("failed to override resolv.conf in tools chroot:\n%w", err)
	}
	t.resolvConf = &resolvConf

	return nil
}

// Chroot returns the underlying safechroot.
func (t *ToolsChroot) Chroot() *safechroot.Chroot {
	return t.chroot
}

// CleanClose restores resolv.conf and closes the chroot, returning the first
// error. Use this in the success path.
func (t *ToolsChroot) CleanClose() error {
	if err := restoreResolvConf(t.ctx, *t.resolvConf, t.chroot); err != nil {
		return fmt.Errorf("failed to restore resolv.conf in tools chroot:\n%w", err)
	}
	t.resolvConf = nil

	if err := t.chroot.Close(true); err != nil {
		return fmt.Errorf("failed to close tools chroot (%s):\n%w", t.toolsDir, err)
	}

	return nil
}

// Close is a best-effort cleanup intended for `defer`. Errors are logged.
// Safe to call on a partially constructed or partially deconstructed instance.
func (t *ToolsChroot) Close() {
	if t.resolvConf != nil {
		if err := restoreResolvConf(t.ctx, *t.resolvConf, t.chroot); err != nil {
			logger.Log.Warnf("Failed to restore resolv.conf in tools chroot (%s): %v", t.toolsDir, err)
		}
		t.resolvConf = nil
	}

	if t.chroot != nil {
		if err := t.chroot.Close(true); err != nil {
			logger.Log.Warnf("Failed to close tools chroot (%s): %v", t.toolsDir, err)
		}
	}
}
