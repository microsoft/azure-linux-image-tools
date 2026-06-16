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

// extractTarLayer applies a tar stream onto destDir with OCI/Docker whiteout handling.
// The caller handles decompression. destDir must exist.
func extractTarLayer(reader io.Reader, destDir string) error {
	tr := tar.NewReader(reader)

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%w: reading tar header:\n%w", ErrExtractFailed, err)
		}

		if err := applyTarEntry(tr, hdr, destDir); err != nil {
			return err
		}
	}
}

func applyTarEntry(tr *tar.Reader, hdr *tar.Header, destDir string) error {
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
		parent := filepath.Join(destDir, filepath.FromSlash(dirRel))
		return removeDirChildren(parent)

	case strings.HasPrefix(base, whiteoutPrefix):
		realName := strings.TrimPrefix(base, whiteoutPrefix)
		target := filepath.Join(destDir, filepath.FromSlash(dirRel), realName)
		return removePathAll(target)
	}

	target := filepath.Join(destDir, filepath.FromSlash(cleanRel))

	switch hdr.Typeflag {
	case tar.TypeDir:
		return writeDir(target, hdr)

	case tar.TypeReg, tar.TypeRegA:
		return writeRegularFile(target, hdr, tr)

	case tar.TypeSymlink:
		return writeSymlink(target, hdr)

	case tar.TypeLink:
		return writeHardlink(target, hdr, destDir)

	default:
		// Skip device/fifo/PAX/sparse entries; tools chroot doesn't need them.
		return nil
	}
}

// safeJoinRel cleans a tar entry name to a slash-separated relative path,
// rejecting absolute paths and ".." traversal.
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

func writeDir(target string, hdr *tar.Header) error {
	mode := os.FileMode(hdr.Mode & 0o7777)

	info, err := os.Lstat(target)
	switch {
	case err == nil && info.IsDir():
		if err := os.Chmod(target, mode); err != nil {
			return fmt.Errorf("%w: chmod dir (%s):\n%w", ErrExtractFailed, target, err)
		}

	case err == nil:
		if err := os.RemoveAll(target); err != nil {
			return fmt.Errorf("%w: removing conflicting entry (%s):\n%w", ErrExtractFailed, target, err)
		}
		if err := os.MkdirAll(target, mode); err != nil {
			return fmt.Errorf("%w: mkdir (%s):\n%w", ErrExtractFailed, target, err)
		}

	case errors.Is(err, os.ErrNotExist):
		if err := os.MkdirAll(target, mode); err != nil {
			return fmt.Errorf("%w: mkdir (%s):\n%w", ErrExtractFailed, target, err)
		}

	default:
		return fmt.Errorf("%w: stat (%s):\n%w", ErrExtractFailed, target, err)
	}

	applyOwnership(target, hdr, false)
	return nil
}

func writeRegularFile(target string, hdr *tar.Header, tr io.Reader) error {
	if err := ensureParentDir(target); err != nil {
		return err
	}
	if err := removeIfNotMatching(target, false); err != nil {
		return err
	}

	mode := os.FileMode(hdr.Mode & 0o7777)
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
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

	if err := os.Chmod(target, mode); err != nil {
		return fmt.Errorf("%w: chmod file (%s):\n%w", ErrExtractFailed, target, err)
	}

	applyOwnership(target, hdr, false)
	return nil
}

func writeSymlink(target string, hdr *tar.Header) error {
	if err := ensureParentDir(target); err != nil {
		return err
	}
	if err := removePathAll(target); err != nil {
		return err
	}

	if err := os.Symlink(hdr.Linkname, target); err != nil {
		return fmt.Errorf("%w: symlink (%s -> %s):\n%w", ErrExtractFailed, target, hdr.Linkname, err)
	}

	applyOwnership(target, hdr, true)
	return nil
}

func writeHardlink(target string, hdr *tar.Header, destDir string) error {
	if err := ensureParentDir(target); err != nil {
		return err
	}

	linkRel, err := safeJoinRel(hdr.Linkname)
	if err != nil {
		return fmt.Errorf("%w: hardlink target (%q):\n%w", ErrExtractFailed, hdr.Linkname, err)
	}
	linkSrc := filepath.Join(destDir, filepath.FromSlash(linkRel))

	if err := removePathAll(target); err != nil {
		return err
	}
	if err := os.Link(linkSrc, target); err != nil {
		return fmt.Errorf("%w: hardlink (%s -> %s):\n%w", ErrExtractFailed, target, linkSrc, err)
	}

	return nil
}

func ensureParentDir(target string) error {
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("%w: mkdir parent (%s):\n%w", ErrExtractFailed, parent, err)
	}
	return nil
}

func removeIfNotMatching(target string, wantDir bool) error {
	info, err := os.Lstat(target)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: stat (%s):\n%w", ErrExtractFailed, target, err)
	}

	if wantDir == info.IsDir() {
		return nil
	}

	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("%w: removing conflicting entry (%s):\n%w", ErrExtractFailed, target, err)
	}
	return nil
}

func removePathAll(target string) error {
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("%w: removing (%s):\n%w", ErrExtractFailed, target, err)
	}
	return nil
}

func removeDirChildren(dir string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: read dir (%s):\n%w", ErrExtractFailed, dir, err)
	}

	for _, entry := range entries {
		child := filepath.Join(dir, entry.Name())
		if err := os.RemoveAll(child); err != nil {
			return fmt.Errorf("%w: removing (%s):\n%w", ErrExtractFailed, child, err)
		}
	}
	return nil
}

func applyOwnership(target string, hdr *tar.Header, isSymlink bool) {
	if hdr.Uid < 0 || hdr.Gid < 0 {
		return
	}
	if isSymlink {
		_ = os.Lchown(target, hdr.Uid, hdr.Gid)
	} else {
		_ = os.Chown(target, hdr.Uid, hdr.Gid)
	}
}
