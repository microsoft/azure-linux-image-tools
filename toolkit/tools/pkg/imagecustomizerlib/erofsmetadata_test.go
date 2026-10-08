// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestVerifyErofsTree(t *testing.T) {
	for _, mutation := range []string{"none", "content", "mode", "xattr", "missing", "extra"} {
		t.Run(mutation, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "source")
			target := filepath.Join(dir, "target")
			require.NoError(t, os.Mkdir(source, 0o755))
			path := filepath.Join(source, "file")
			require.NoError(t, os.WriteFile(path, []byte("content"), 0o640))
			require.NoError(t, unix.Setxattr(path, "user.proof", []byte("keep"), 0))
			require.NoError(t, shell.ExecuteLive(true, "cp", "-a", "--preserve=all", source, target))
			destination := filepath.Join(target, "file")
			info, err := os.Lstat(destination)
			require.NoError(t, err)
			switch mutation {
			case "content":
				require.NoError(t, os.WriteFile(destination, []byte("changed"), 0o640))
				require.NoError(t, os.Chtimes(destination, info.ModTime(), info.ModTime()))
			case "mode":
				require.NoError(t, os.Chmod(destination, 0o600))
			case "xattr":
				require.NoError(t, unix.Setxattr(destination, "user.proof", []byte("lost"), 0))
			case "missing":
				require.NoError(t, os.Remove(destination))
			case "extra":
				require.NoError(t, os.WriteFile(filepath.Join(target, "extra"), nil, 0o600))
			}
			err = verifyErofsTree(source, target)
			if mutation == "none" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
