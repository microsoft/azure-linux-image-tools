// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package tarutils

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
)

func CreateTarGzArchive(sourceDir, outputArchivePath string) (err error) {
	logger.Log.Infof("Creating archive (%s) from (%s)", outputArchivePath, sourceDir)

	outFile, err := os.Create(outputArchivePath)
	if err != nil {
		return fmt.Errorf("failed to create archive (%s):\n%w", outputArchivePath, err)
	}
	defer func() {
		closeErr := outFile.Close()
		if err == nil {
			err = closeErr
		}
	}()

	gw := gzip.NewWriter(outFile)
	defer func() {
		closeErr := gw.Close()
		if err == nil {
			err = closeErr
		}
	}()

	tw := tar.NewWriter(gw)
	defer func() {
		closeErr := tw.Close()
		if err == nil {
			err = closeErr
		}
	}()

	err = filepath.Walk(sourceDir, func(file string, info os.FileInfo, walkErr error) (err error) {
		if walkErr != nil {
			return walkErr
		}

		// For symlinks, capture the link target so it is preserved as a symlink in the archive rather than
		// dereferenced. os.Open() would otherwise follow the link, and copying a followed directory fails.
		linkTarget := ""
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err = os.Readlink(file)
			if err != nil {
				return err
			}
		}

		header, err := tar.FileInfoHeader(info, linkTarget)
		if err != nil {
			return err
		}

		// Adjust the header name to maintain folder structure
		relPath, err := filepath.Rel(sourceDir, file)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath) // Ensure forward slashes

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// Only regular files have contents to copy. Directories, symlinks, and other special files have no body.
		if !info.Mode().IsRegular() {
			return nil
		}

		// Write file contents
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer func() {
			closeErr := f.Close()
			if err == nil {
				err = closeErr
			}
		}()

		_, err = io.Copy(tw, f)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create archive (%s):\n%w", outputArchivePath, err)
	}

	return nil
}

func ExpandTarGzArchive(sourceArchivePath, outputDir string) (err error) {
	logger.Log.Infof("Expanding archive (%s) to (%s)", sourceArchivePath, outputDir)

	f, err := os.Open(sourceArchivePath)
	if err != nil {
		return fmt.Errorf("failed to archive (%s):\n%w", sourceArchivePath, err)
	}
	defer func() {
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
	}()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader for (%s):\n%w", sourceArchivePath, err)
	}
	defer func() {
		closeErr := gzr.Close()
		if err == nil {
			err = closeErr
		}
	}()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read header from archive:\n%w", err)
		}

		// Ensure the name is not a directory traversal element (e.g. '..') or
		// an absolute path. We call filepath.Clean() to normalize it before
		// checking.
		cleanName := filepath.Clean(header.Name)
		if strings.Contains(cleanName, "..") || filepath.IsAbs(cleanName) {
			return fmt.Errorf("unallowed file reference in archive. (%s) may reference a file outside the expansion root (%s)", header.Name, outputDir)
		}

		target := filepath.Join(outputDir, cleanName)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create folder (%s)\n%w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create parent folder for (%s)\n%w", target, err)
			}
			outFile, err := os.Create(target)
			if err != nil {
				return fmt.Errorf("failed to create (%s):\n%w", target, err)
			}
			_, err = io.Copy(outFile, tr)
			if err != nil {
				// If this fails, we will still report the original error from
				// the io.Copy()
				outFile.Close()
				return fmt.Errorf("failed to copy (%s) from archive:\n%w", target, err)
			}
			err = outFile.Close()
			if err != nil {
				return fmt.Errorf("failed to close (%s):\n%w", target, err)
			}

			if err := os.Chmod(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to set permissions (%d) on (%s):\n%w", os.FileMode(header.Mode), target, err)
			}
		case tar.TypeSymlink:
			// Matching the header.Name check above to ensure the link target doesn't point outside the expansion root.
			if cleanLink := filepath.Clean(header.Linkname); strings.Contains(cleanLink, "..") || filepath.IsAbs(cleanLink) {
				return fmt.Errorf("unallowed symlink in archive. (%s -> %s) may reference a file outside the expansion root (%s)",
					header.Name, header.Linkname, outputDir)
			}

			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create parent folder for (%s)\n%w", target, err)
			}
			// Remove any pre-existing entry so re-extraction is idempotent (os.Symlink fails if the target exists).
			if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to clear existing path (%s):\n%w", target, err)
			}
			if err := os.Symlink(header.Linkname, target); err != nil {
				return fmt.Errorf("failed to create symlink (%s -> %s):\n%w", target, header.Linkname, err)
			}
		default:
			return fmt.Errorf("failed to process unsupported file type in archive (%s): (%v)", target, header.Typeflag)
		}
	}
	return nil
}
