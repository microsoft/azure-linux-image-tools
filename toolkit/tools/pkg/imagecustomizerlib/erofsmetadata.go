// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"golang.org/x/sys/unix"
)

// cp --preserve=all can succeed after losing extended attributes. Verify the
// staged and packed trees before accepting changes to a sealed filesystem.
func verifyErofsTree(source string, destination string) error {
	type inode struct{ device, number uint64 }
	hardlinks := map[inode]inode{}
	paths := map[string]struct{}{}
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		paths[relative] = struct{}{}
		target := filepath.Join(destination, relative)
		var before, after unix.Stat_t
		if err := unix.Lstat(path, &before); err != nil {
			return err
		}
		if err := unix.Lstat(target, &after); err != nil {
			return err
		}
		if before.Mode != after.Mode || before.Uid != after.Uid || before.Gid != after.Gid ||
			before.Mtim != after.Mtim || before.Rdev != after.Rdev {
			return fmt.Errorf("EROFS metadata differs (%s)", relative)
		}
		sourceAttrs, err := readErofsXattrs(path)
		if err != nil {
			return err
		}
		targetAttrs, err := readErofsXattrs(target)
		if err != nil {
			return err
		}
		if !maps.Equal(sourceAttrs, targetAttrs) {
			return fmt.Errorf("EROFS extended attributes differ (%s)", relative)
		}
		switch before.Mode & unix.S_IFMT {
		case unix.S_IFREG:
			if before.Nlink != after.Nlink {
				return fmt.Errorf("EROFS hardlink count differs (%s)", relative)
			}
			sourceHash, err := file.GenerateSHA256(path)
			if err != nil {
				return err
			}
			targetHash, err := file.GenerateSHA256(target)
			if err != nil {
				return err
			}
			if sourceHash != targetHash {
				return fmt.Errorf("EROFS file contents differ (%s)", relative)
			}
			key := inode{uint64(before.Dev), before.Ino}
			value := inode{uint64(after.Dev), after.Ino}
			if previous, exists := hardlinks[key]; exists && previous != value {
				return fmt.Errorf("EROFS hardlink identity differs (%s)", relative)
			}
			hardlinks[key] = value
		case unix.S_IFLNK:
			beforeTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			afterTarget, err := os.Readlink(target)
			if err != nil {
				return err
			}
			if beforeTarget != afterTarget {
				return fmt.Errorf("EROFS symlink target differs (%s)", relative)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return filepath.WalkDir(destination, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(destination, path)
		if err != nil {
			return err
		}
		if _, exists := paths[relative]; !exists {
			return fmt.Errorf("unexpected EROFS path (%s)", relative)
		}
		return nil
	})
}

func readErofsXattrs(path string) (map[string]string, error) {
	size, err := unix.Llistxattr(path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list extended attributes (%s):\n%w", path, err)
	}
	names := make([]byte, size)
	n, err := unix.Llistxattr(path, names)
	if err != nil {
		return nil, err
	}
	attributes := map[string]string{}
	for _, name := range strings.Split(string(names[:n]), "\x00") {
		if name == "" {
			continue
		}
		size, err := unix.Lgetxattr(path, name, nil)
		if err != nil {
			return nil, err
		}
		value := make([]byte, size)
		n, err := unix.Lgetxattr(path, name, value)
		if err != nil {
			return nil, err
		}
		attributes[name] = string(value[:n])
	}
	return attributes, nil
}
