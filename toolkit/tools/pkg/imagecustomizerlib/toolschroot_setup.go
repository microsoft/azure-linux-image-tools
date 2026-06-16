// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/pkg/imagecustomizerlib/toolschroot"
)

var ErrToolsChrootCacheDirUnresolved = NewImageCustomizerError(
	"ToolsChroot:CacheDirUnresolved",
	"could not determine a cache directory for the auto-provisioned tools chroot")

const (
	toolsChrootCacheSubdir           = "toolschroot"
	defaultToolsChrootCacheNamespace = "imagecustomizer"
)

// resolveToolsChrootCacheDir picks the cache root for auto-provisioned tools chroots, preferring
// --image-cache-dir, then $XDG_CACHE_HOME, then $HOME/.cache.
func resolveToolsChrootCacheDir(options ImageCustomizerOptions) (string, error) {
	if options.ImageCacheDir != "" {
		return filepath.Join(options.ImageCacheDir, toolsChrootCacheSubdir), nil
	}

	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, defaultToolsChrootCacheNamespace, toolsChrootCacheSubdir), nil
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("%w: no --image-cache-dir, XDG_CACHE_HOME, or HOME available", ErrToolsChrootCacheDirUnresolved)
	}

	return filepath.Join(home, ".cache", defaultToolsChrootCacheNamespace, toolsChrootCacheSubdir), nil
}

// setupToolsChroot provisions the tools-chroot rootfs (pulling the OCI container on cache miss),
// copies it into <buildDir>/toolsroot/, and returns an initialized safechroot. The caller owns
// the returned chroot and must Close it.
func setupToolsChroot(ctx context.Context, rc *ResolvedConfig, distroHandler DistroHandler,
) (*safechroot.Chroot, error) {
	cacheDir, err := resolveToolsChrootCacheDir(rc.Options)
	if err != nil {
		return nil, err
	}

	provisionedDir, err := toolschroot.Provision(ctx, distroHandler.GetTargetOs(), cacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to provision tools chroot:\n%w", err)
	}

	toolsChrootDir := filepath.Join(rc.BuildDirAbs, toolsRoot)
	logger.Log.Debugf("Tools chroot: staging at (%s)", toolsChrootDir)

	if _, statErr := os.Stat(toolsChrootDir); statErr == nil {
		return nil, fmt.Errorf("tools chroot directory (%s) already exists", toolsChrootDir)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return nil, fmt.Errorf("stat tools chroot directory (%s):\n%w", toolsChrootDir, statErr)
	}

	if err := os.MkdirAll(toolsChrootDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create tools chroot dir (%s):\n%w", toolsChrootDir, err)
	}
	// `cp -a` preserves the usrmerge symlinks (/bin -> usr/bin, etc.) that file.CopyDir would
	// flatten into broken files.
	if err := shell.ExecuteLive(true, "cp", "-a", provisionedDir+"/.", toolsChrootDir+"/"); err != nil {
		return nil, fmt.Errorf("failed to copy tools chroot rootfs to (%s):\n%w", toolsChrootDir, err)
	}

	chroot := safechroot.NewChroot(toolsChrootDir, true)
	if err := chroot.Initialize("", nil, nil, true); err != nil {
		return nil, fmt.Errorf("failed to initialize tools chroot:\n%w", err)
	}

	return chroot, nil
}
