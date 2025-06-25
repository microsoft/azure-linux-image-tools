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
)

const (
	// The cpio archive root will become the OS root when the OS boots.
	// The root needs to be 0755 of else many processes fail to start because
	// of incorrect permissions.
	initrdRootDirPermissions = 0755
)

func CreateInitrdImageFromFolder(inputRootDir, outputInitrdImagePath string) (err error) {
	outputFile, err := os.Create(outputInitrdImagePath)
	if err != nil {
		return fmt.Errorf("failed to create image file (%s):\n%w", outputInitrdImagePath, err)
	}
	defer func() {
		closeErr := outputFile.Close()
		if err != nil {
			err = closeErr
		}
	}()

	gzipWriter := pgzip.NewWriter(outputFile)
	defer func() {
		closeErr := gzipWriter.Close()
		if err != nil {
			err = closeErr
		}
	}()

	cpioWriter := cpio.NewWriter(gzipWriter)
	defer func() {
		closeErr := cpioWriter.Close()
		if err != nil {
			err = closeErr
		}
	}()

	// Traverse the directory structure and add all the files/directories/links to the archive.
	processedRoot := false
	err = filepath.Walk(inputRootDir, func(path string, info os.FileInfo, fileErr error) (err error) {
		if fileErr != nil {
			return fmt.Errorf("encountered a file walk error on path (%s):\n%w", path, fileErr)
		}

		isRoot := false
		if !processedRoot {
			if inputRootDir == path {
				processedRoot = true
				isRoot = true
			}
		}

		err = addFileToCpioArchive(inputRootDir, path, info, isRoot, cpioWriter)
		if err != nil {
			return fmt.Errorf("failed to add (%s) to archive (%s):\n%w", path, outputInitrdImagePath, err)
		}
		return nil
	})

	return nil
}

func buildCpioHeader(inputRootDir, path string, info os.FileInfo, isRoot bool, link string) (cpioHeader *cpio.Header, err error) {
	// Convert the OS header into a CPIO header
	cpioHeader, err = cpio.FileInfoHeader(info, link)
	if err != nil {
		return nil, fmt.Errorf("failed to convert OS file info into a cpio header for (%s)\n%w", path, err)
	}

	if isRoot {
		cpioHeader.Mode = (cpioHeader.Mode & ^cpio.ModePerm) | cpio.FileMode(initrdRootDirPermissions)
	}

	// Convert full path to relative path
	relPath, err := filepath.Rel(inputRootDir, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get relative path of (%s) using root (%s):\n%w", path, inputRootDir, err)
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

func addFileToCpioArchive(inputRootDir, path string, info os.FileInfo, isRoot bool, cpioWriter *cpio.Writer) (err error) {
	var link string
	if info.Mode()&os.ModeSymlink != 0 {
		link, err = os.Readlink(path)
		if err != nil {
			return fmt.Errorf("failed to read link information of (%s):\n%w", path, err)
		}
	}

	cpioHeader, err := buildCpioHeader(inputRootDir, path, info, isRoot, link)
	if err != nil {
		return fmt.Errorf("failed to construct cpio file header for (%s)\n%w", path, err)
	}

	err = cpioWriter.WriteHeader(cpioHeader)
	if err != nil {
		return fmt.Errorf("failed to write cpio header for (%s)\n%w", path, err)
	}

	switch {
	case info.Mode().IsRegular():
		fileToAdd, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open (%s)\n%w", path, err)
		}
		defer func() {
			closeErr := fileToAdd.Close()
			if err != nil {
				err = closeErr
			}
		}()

		_, err = io.Copy(cpioWriter, fileToAdd)
		if err != nil {
			return fmt.Errorf("failed to write (%s) to cpio archive\n%w", path, err)
		}
	case info.Mode()&os.ModeSymlink != 0:
		_, err = cpioWriter.Write([]byte(link))
		if err != nil {
			return fmt.Errorf("failed to write link (%s)\n%w", path, err)
		}
	default:
		// For all other special files, they will be of size 0 and only contain
		// the header in the archive.
	}

	return nil
}

// The golang implementation maps the setuid/setgid/sticky flags to bits
// different from those defined in native C implementation. As a result,
// we must convert between them.
// See https://github.com/golang/go/blob/release-branch.go1.24/src/os/stat_js.go
func sysFileModeToGoFileMode(osFileMode uint32) os.FileMode {
	golangFileMode := os.FileMode(osFileMode).Perm()
	if osFileMode&syscall.S_ISUID != 0 {
		golangFileMode |= os.ModeSetuid
	}
	if osFileMode&syscall.S_ISGID != 0 {
		golangFileMode |= os.ModeSetgid
	}
	if osFileMode&syscall.S_ISVTX != 0 {
		golangFileMode |= os.ModeSticky
	}
	return golangFileMode
}

func CreateFolderFromInitrdImage(inputInitrdImagePath, outputDir string) error {
	inputInitrdImageFile, err := os.Open(inputInitrdImagePath)
	if err != nil {
		return fmt.Errorf("failed to open file (%s):\n%w", inputInitrdImagePath, err)
	}
	defer func() {
		closeErr := inputInitrdImageFile.Close()
		if err != nil {
			err = closeErr
		}
	}()

	pgzipReader, err := pgzip.NewReader(inputInitrdImageFile)
	if err != nil {
		return fmt.Errorf("failed to create a pgzip reader for (%s):\n%w", inputInitrdImagePath, err)
	}
	defer func() {
		closeErr := pgzipReader.Close()
		if err != nil {
			err = closeErr
		}
	}()

	cpioReader := cpio.NewReader(pgzipReader)
	for {
		cpioHeader, err := cpioReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read cpio header from (%s):\n%w", inputInitrdImagePath, err)
		}

		path := filepath.Join(outputDir, cpioHeader.Name)
		cpioFileMode := cpioHeader.Mode & (cpio.ModePerm | cpio.ModeSetuid | cpio.ModeSetgid | cpio.ModeSticky)
		golangFileMode := sysFileModeToGoFileMode(uint32(cpioFileMode))
		fileType := cpioHeader.Mode & cpio.ModeType

		switch fileType {
		case cpio.ModeDir:
			err := os.MkdirAll(path, golangFileMode)
			if err != nil {
				return fmt.Errorf("failed to create directory (%s):\n%w", path, err)
			}

			err = os.Chown(path, cpioHeader.UID, cpioHeader.GID)
			if err != nil {
				return fmt.Errorf("failed to set ownership on extracted (%s) to (%d,%d):\n%w", path, cpioHeader.UID, cpioHeader.GID, err)
			}

			// UID/GID/Sticky permissions must be set after chown or else they
			// do not take effect (MkdirAll(,permissions) is not sufficient in
			// this case).
			err = os.Chmod(path, golangFileMode)
			if err != nil {
				return fmt.Errorf("failed to set permissions on extracted (%s) to (0%08o):\n%w", path, golangFileMode, err)
			}

		case cpio.ModeRegular:
			destFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, golangFileMode)
			if err != nil {
				return fmt.Errorf("failed to create file (%s):\n%w", path, err)
			}
			_, err = io.Copy(destFile, cpioReader)
			if err != nil {
				// If destFile.Close() fails, we will still report the original
				// error from the io.Copy()
				destFile.Close()
				return fmt.Errorf("failed to write file (%s):\n%w", path, err)
			}
			err = destFile.Close()
			if err != nil {
				return fmt.Errorf("failed to close (%s):\n%w", path, err)
			}

			err = os.Chown(path, cpioHeader.UID, cpioHeader.GID)
			if err != nil {
				return fmt.Errorf("failed to set ownership on extracted (%s) to (%d,%d):\n%w", path, cpioHeader.UID, cpioHeader.GID, err)
			}

			// UID/GID/Sticky permissions must be set after chown or else they
			// do not take effect (OpenFile(,permissions) is not sufficient in
			// this case).
			err = os.Chmod(path, golangFileMode)
			if err != nil {
				return fmt.Errorf("failed to set permissions on extracted (%s) to (0%08o):\n%w", path, golangFileMode, err)
			}

		case cpio.ModeSymlink:
			err = os.Symlink(cpioHeader.Linkname, path)
			if err != nil {
				return fmt.Errorf("failed to create symbolic link (%s) to (%s)\n%w", cpioHeader.Linkname, path, err)
			}
		default:
			return fmt.Errorf("unsupported type (%s) in cpio archive (%s)", fileType, inputInitrdImagePath)
		}
	}

	return nil
}
