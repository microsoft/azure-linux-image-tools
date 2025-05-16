// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package initrdutils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/cavaliercoder/go-cpio"
	"github.com/klauspost/pgzip"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
)

func CreateInitrdImageFromFolder(inputDir, outputInitrdImagePath string) (err error) {
	// The folder permissions will become the `/` permissions when the initrd is
	// mounted. This needs to be 0755 or some processes will fail to function
	// correctly.
	err = os.Chmod(inputDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to change folder permissions for (%s):\n%w", inputDir, err)
	}

	// Create the image, the compressor, and the cpio writers.
	outputFile, err := os.Create(outputInitrdImagePath)
	if err != nil {
		return fmt.Errorf("failed to create image file (%s):\n%w", outputInitrdImagePath, err)
	}
	defer outputFile.Close()

	gzipWriter := pgzip.NewWriter(outputFile)
	defer gzipWriter.Close()

	cpioWriter := cpio.NewWriter(gzipWriter)
	defer func() {
		closeErr := cpioWriter.Close()
		if err != nil {
			err = closeErr
		}
	}()

	// Traverse the directory structure and add all the files/directories/links to the archive.
	err = filepath.Walk(inputDir, func(path string, info os.FileInfo, fileErr error) (err error) {
		if fileErr != nil {
			return fmt.Errorf("encountered a file walk error on path (%s):\n%w", path, fileErr)
		}
		err = addFileToCpioArchive(inputDir, path, info, cpioWriter)
		if err != nil {
			return fmt.Errorf("failed to add (%s) to archive:\n%w", path, err)
		}
		return nil
	})

	return nil
}

func buildCpioHeader(inputDir, path string, info os.FileInfo, link string) (cpioHeader *cpio.Header, err error) {
	// Convert the OS header into a CPIO header
	cpioHeader, err = cpio.FileInfoHeader(info, link)
	if err != nil {
		return nil, fmt.Errorf("failed to convert OS file info into a cpio header for (%s)\n%w", path, err)
	}

	// Convert full path to relative path
	relPath, err := filepath.Rel(inputDir, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get relative path of (%s) using root (%s):\n%w", path, inputDir, err)
	}
	cpioHeader.Name = relPath

	// Set owners (cpio.FileInfoHeader() does not set the owners)
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("failed to get file stat of (%s)", path)
	}
	cpioHeader.UID = int(stat.Uid)
	cpioHeader.GID = int(stat.Gid)

	return cpioHeader, nil
}

func addFileToCpioArchive(inputDir, path string, info os.FileInfo, cpioWriter *cpio.Writer) (err error) {
	var link string
	if info.Mode()&os.ModeSymlink != 0 {
		link, err = os.Readlink(path)
		if err != nil {
			return fmt.Errorf("failed to read link information of (%s):\n%w", path, err)
		}
	}

	cpioHeader, err := buildCpioHeader(inputDir, path, info, link)
	if err != nil {
		return fmt.Errorf("failed to construct cpio file header for (%s)\n%w", path, err)
	}

	err = cpioWriter.WriteHeader(cpioHeader)
	if err != nil {
		return fmt.Errorf("failed to write cpio header for (%s)\n%w", path, err)
	}

	if info.Mode().IsRegular() {
		fileToAdd, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open (%s)\n%w", path, err)
		}
		defer fileToAdd.Close()

		_, err = io.Copy(cpioWriter, fileToAdd)
		if err != nil {
			return fmt.Errorf("failed to write (%s) to cpio archive\n%w", path, err)
		}
	} else {
		if info.Mode()&os.ModeSymlink != 0 {
			_, err = cpioWriter.Write([]byte(link))
			if err != nil {
				return fmt.Errorf("failed to write link (%s)\n%w", path, err)
			}
		}

		// For all other special files, they will be of size 0 and only contain
		// the header in the archive.
	}

	return nil
}

func updateFileOwnership(path string, fileMode os.FileMode, uid, gid int) (err error) {
	err = os.Chown(path, uid, gid)
	if err != nil {
		return fmt.Errorf("failed to set ownership on extracted (%s) to (%d,%d):\n%w", path, uid, gid, err)
	}

	if fileMode&cpio.ModePerm != 0 {
		// os.Chmod() does not set the higher-order bits (setuid, setgid, and
		// setsticky). So, we are calling the `chmod` command-line instead.
		fileModeString := fmt.Sprintf("%#o", fileMode)
		chmodParams := []string{
			fileModeString,
			path,
		}
		err = shell.ExecuteLive(true /*squashErrors*/, "chmod", chmodParams...)
		if err != nil {
			return fmt.Errorf("failed to change mode of file (%s) to (%s): %w", fileModeString, path, err)
		}
	}
	return nil
}

func CreateFolderFromInitrdImage(inputInitrdImagePath, outputDir string) (err error) {
	// Open the input archive
	inputInitrdImageFile, err := os.Open(inputInitrdImagePath)
	if err != nil {
		return fmt.Errorf("failed to open archive(%s):\n%w", inputInitrdImagePath, err)
	}
	defer inputInitrdImageFile.Close()

	// Create pgzip reader
	pgzipReader, err := pgzip.NewReader(inputInitrdImageFile)
	if err != nil {
		return fmt.Errorf("create pgzip reader: %w", err)
	}
	defer pgzipReader.Close()

	// Create cpio reader
	cpioReader := cpio.NewReader(pgzipReader)

	for {
		cpioHeader, err := cpioReader.Next()
		if err == io.EOF {
			break // end of archive
		}
		if err != nil {
			return fmt.Errorf("failed to read cpio header from (%s):\n%w", inputInitrdImagePath, err)
		}

		path := filepath.Join(outputDir, cpioHeader.Name)
		fileMode := os.FileMode(cpioHeader.Mode & (cpio.ModePerm | cpio.ModeSetuid | cpio.ModeSetgid | cpio.ModeSticky))
		fileType := cpioHeader.Mode & cpio.ModeType

		switch fileType {
		case cpio.ModeDir:
			err := os.MkdirAll(path, fileMode)
			if err != nil {
				return fmt.Errorf("failed to create directory %s: %w", path, err)
			}

			err = updateFileOwnership(path, fileMode, cpioHeader.UID, cpioHeader.GID)
			if err != nil {
				return fmt.Errorf("failed to update ownership of (%s)\n%w", path, err)
			}

		case cpio.ModeRegular:
			destFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode)
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", path, err)
			}
			_, err = io.Copy(destFile, cpioReader)
			destFile.Close()
			if err != nil {
				return fmt.Errorf("write file %s: %w", path, err)
			}

			err = updateFileOwnership(path, fileMode, cpioHeader.UID, cpioHeader.GID)
			if err != nil {
				return fmt.Errorf("failed to update ownership of (%s)\n%w", path, err)
			}

		case cpio.ModeSymlink:
			os.Symlink(cpioHeader.Linkname, path)
		default:
			return fmt.Errorf("unsupported unknown type %#o in CPIO archive.", fileType)
		}
	}

	return nil
}
