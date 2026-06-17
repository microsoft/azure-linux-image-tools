// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package toolschroot

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var ErrExtractFailed = errors.New("failed to extract OCI image layer")

const (
	whiteoutPrefix     = ".wh."
	opaqueWhiteoutName = ".wh..wh..opq"
)

// extractTarLayer applies a tar stream onto destDir with OCI/Docker whiteout
// handling. destDir must exist; caller handles decompression. All filesystem
// ops go through os.Root to block Zip Slip and symlink-chain escapes.
func extractTarLayer(reader io.Reader, destDir string) error {
	root, err := os.OpenRoot(destDir)
	if err != nil {
		return fmt.Errorf("%w: opening destination root (%s):\n%w", ErrExtractFailed, destDir, err)
	}
	defer root.Close()

	tr := tar.NewReader(reader)

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%w: reading tar header:\n%w", ErrExtractFailed, err)
		}

		if err := applyTarEntry(tr, hdr, root); err != nil {
			return err
		}
	}
}

func applyTarEntry(tr *tar.Reader, hdr *tar.Header, root *os.Root) error {
	cleanRel, err := safeJoinRel(hdr.Name)
	if err != nil {
		return fmt.Errorf("%w: unsafe entry path (%q):\n%w", ErrExtractFailed, hdr.Name, err)
	}

	base := path.Base(cleanRel)
	dirRel := path.Dir(cleanRel)
	if dirRel == "." {
		dirRel = ""
	}

	switch {
	case base == opaqueWhiteoutName:
		parent := filepath.FromSlash(dirRel)
		if parent == "" {
			parent = "."
		}
		return removeDirChildren(root, parent)

	case strings.HasPrefix(base, whiteoutPrefix):
		realName := strings.TrimPrefix(base, whiteoutPrefix)
		target := filepath.Join(filepath.FromSlash(dirRel), realName)
		return removePathAll(root, target)
	}

	target := filepath.FromSlash(cleanRel)

	switch hdr.Typeflag {
	case tar.TypeDir:
		return writeDir(root, target, hdr)

	case tar.TypeReg, tar.TypeRegA:
		return writeRegularFile(root, target, hdr, tr)

	case tar.TypeSymlink:
		return writeSymlink(root, target, hdr)

	case tar.TypeLink:
		return writeHardlink(root, target, hdr)

	default:
		// Skip device/fifo/PAX/sparse entries; tools chroot doesn't need them.
		return nil
	}
}

// safeJoinRel cleans a tar entry name to a slash-separated relative path,
// rejecting empty names and ".." that escapes after cleaning.
func safeJoinRel(name string) (string, error) {
	if name == "" {
		return "", errors.New("empty entry name")
	}

	clean := path.Clean("/" + name)
	clean = strings.TrimPrefix(clean, "/")

	if clean == "" || clean == "." {
		return "", errors.New("entry resolves to empty path")
	}

	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("path traversal: %q", name)
	}

	return clean, nil
}

func writeDir(root *os.Root, target string, hdr *tar.Header) error {
	mode := os.FileMode(hdr.Mode & 0o7777)

	info, err := root.Lstat(target)
	switch {
	case err == nil && info.IsDir():
		if err := root.Chmod(target, mode); err != nil {
			return fmt.Errorf("%w: chmod dir (%s):\n%w", ErrExtractFailed, target, err)
		}

	case err == nil:
		if err := root.RemoveAll(target); err != nil {
			return fmt.Errorf("%w: removing conflicting entry (%s):\n%w", ErrExtractFailed, target, err)
		}
		if err := root.MkdirAll(target, mode); err != nil {
			return fmt.Errorf("%w: mkdir (%s):\n%w", ErrExtractFailed, target, err)
		}

	case errors.Is(err, os.ErrNotExist):
		if err := root.MkdirAll(target, mode); err != nil {
			return fmt.Errorf("%w: mkdir (%s):\n%w", ErrExtractFailed, target, err)
		}

	default:
		return fmt.Errorf("%w: stat (%s):\n%w", ErrExtractFailed, target, err)
	}

	applyOwnership(root, target, hdr, false)
	return nil
}

func writeRegularFile(root *os.Root, target string, hdr *tar.Header, tr io.Reader) error {
	if err := ensureParentDir(root, target); err != nil {
		return err
	}
	if err := removeIfNotMatching(root, target, false); err != nil {
		return err
	}

	mode := os.FileMode(hdr.Mode & 0o7777)
	out, err := root.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("%w: create file (%s):\n%w", ErrExtractFailed, target, err)
	}

	if _, err := io.Copy(out, tr); err != nil {
		out.Close()
		return fmt.Errorf("%w: write file (%s):\n%w", ErrExtractFailed, target, err)
	}

	if err := out.Close(); err != nil {
		return fmt.Errorf("%w: close file (%s):\n%w", ErrExtractFailed, target, err)
	}

	if err := root.Chmod(target, mode); err != nil {
		return fmt.Errorf("%w: chmod file (%s):\n%w", ErrExtractFailed, target, err)
	}

	applyOwnership(root, target, hdr, false)
	return nil
}

func writeSymlink(root *os.Root, target string, hdr *tar.Header) error {
	if err := ensureParentDir(root, target); err != nil {
		return err
	}
	if err := removePathAll(root, target); err != nil {
		return err
	}

	// Store linkname verbatim. os.Root blocks any later op that would traverse
	// a symlink escaping destDir, so malicious linknames cannot redirect writes.
	if err := root.Symlink(hdr.Linkname, target); err != nil {
		return fmt.Errorf("%w: symlink (%s -> %s):\n%w", ErrExtractFailed, target, hdr.Linkname, err)
	}

	applyOwnership(root, target, hdr, true)
	return nil
}

func writeHardlink(root *os.Root, target string, hdr *tar.Header) error {
	if err := ensureParentDir(root, target); err != nil {
		return err
	}

	linkRel, err := safeJoinRel(hdr.Linkname)
	if err != nil {
		return fmt.Errorf("%w: hardlink target (%q):\n%w", ErrExtractFailed, hdr.Linkname, err)
	}
	linkSrc := filepath.FromSlash(linkRel)

	if err := removePathAll(root, target); err != nil {
		return err
	}
	if err := root.Link(linkSrc, target); err != nil {
		return fmt.Errorf("%w: hardlink (%s -> %s):\n%w", ErrExtractFailed, target, linkSrc, err)
	}

	return nil
}

func ensureParentDir(root *os.Root, target string) error {
	parent := filepath.Dir(target)
	if parent == "." || parent == "" {
		return nil
	}
	if err := root.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("%w: mkdir parent (%s):\n%w", ErrExtractFailed, parent, err)
	}
	return nil
}

// removeIfNotMatching removes a pre-existing entry whose file type does not
// match the entry being written, including any symlink when wantDir is false.
func removeIfNotMatching(root *os.Root, target string, wantDir bool) error {
	info, err := root.Lstat(target)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: stat (%s):\n%w", ErrExtractFailed, target, err)
	}

	isSymlink := info.Mode()&os.ModeSymlink != 0
	if !isSymlink && wantDir == info.IsDir() {
		return nil
	}

	if err := root.RemoveAll(target); err != nil {
		return fmt.Errorf("%w: removing conflicting entry (%s):\n%w", ErrExtractFailed, target, err)
	}
	return nil
}

func removePathAll(root *os.Root, target string) error {
	if err := root.RemoveAll(target); err != nil {
		return fmt.Errorf("%w: removing (%s):\n%w", ErrExtractFailed, target, err)
	}
	return nil
}

func removeDirChildren(root *os.Root, dir string) error {
	dirFile, err := root.Open(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: open dir (%s):\n%w", ErrExtractFailed, dir, err)
	}
	entries, readErr := dirFile.ReadDir(-1)
	dirFile.Close()
	if readErr != nil {
		return fmt.Errorf("%w: read dir (%s):\n%w", ErrExtractFailed, dir, readErr)
	}

	for _, entry := range entries {
		child := filepath.Join(dir, entry.Name())
		if err := root.RemoveAll(child); err != nil {
			return fmt.Errorf("%w: removing (%s):\n%w", ErrExtractFailed, child, err)
		}
	}
	return nil
}

func applyOwnership(root *os.Root, target string, hdr *tar.Header, isSymlink bool) {
	if hdr.Uid < 0 || hdr.Gid < 0 {
		return
	}
	if isSymlink {
		_ = root.Lchown(target, hdr.Uid, hdr.Gid)
	} else {
		_ = root.Chown(target, hdr.Uid, hdr.Gid)
	}
}
