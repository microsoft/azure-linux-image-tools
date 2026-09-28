// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"path/filepath"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
)

// secureJoinRoot resolves a root-relative path within root, clamping any symlink (absolute
// or '..'-escaping, from the base image or an earlier customization) so the result can
// never point outside root. Use it instead of filepath.Join(root, path) for destinations
// that are subsequently written into a mounted image or partition that is not accessed
// through a safechroot.Chroot.
func secureJoinRoot(root, path string) (string, error) {
	dest, err := securejoin.SecureJoin(root, path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve destination (%s) within root:\n%w", path, err)
	}
	// A difference from the lexical join means a symlink on the path was traversed or an
	// escape was clamped; surface it for diagnostics.
	if dest != filepath.Join(root, path) {
		logger.Log.Warnf("destination (%s) under root (%s) resolved through a symlink to (%s)", path, root, dest)
	}
	return dest, nil
}
