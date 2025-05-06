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

	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
)

func CreateTarGzArchive(sourceDir, outputArchivePath string) error {
	logger.Log.Infof("Creating archive (%s) from (%s)", outputArchivePath, sourceDir)

	outFile, err := os.Create(outputArchivePath)
	if err != nil {
		return fmt.Errorf("failed to create archive (%s):\n%w", outputArchivePath, err)
	}
	defer outFile.Close()

	gw := gzip.NewWriter(outFile)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	err = filepath.Walk(sourceDir, func(file string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, info.Name())
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

		// If it's a directory, nothing more to do
		if info.IsDir() {
			return nil
		}

		// Write file contents
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(tw, f)
		return err
	})

	if err != nil {
		return fmt.Errorf("failed to create archive (%s):\n%w", outputArchivePath, err)
	}

	return nil
}

func ExpandTarGzArchive(sourceArchivePath, outputDir string) error {
	logger.Log.Infof("Expanding archive (%s) to (%s)", sourceArchivePath, outputDir)

	f, err := os.Open(sourceArchivePath)
	if err != nil {
		return fmt.Errorf("failed to archive (%s):\n%w", sourceArchivePath, err)
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader for (%s):\n%w", sourceArchivePath, err)
	}
	defer gzr.Close()

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
			defer outFile.Close()
			_, err = io.Copy(outFile, tr)
			if err != nil {
				return fmt.Errorf("failed to copy (%s) from archive:\n%w", target, err)
			}
			outFile.Close()

			if err := os.Chmod(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to set permissions (%d) on (%s):\n%w", os.FileMode(header.Mode), target, err)
			}
		default:
			return fmt.Errorf("failed to process unsupported file type in archive (%s): (%v)", target, header.Typeflag)
		}
	}
	return nil
}
