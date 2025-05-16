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

	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
)

func CreateInitrdImageFromFolder(inputDir, outputInitrdImagePath string) error {
	err := os.Chmod(inputDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to change directory mode of (%s):\n%w", inputDir, err)
	}

	outputFile, err := os.Create(outputInitrdImagePath)
	if err != nil {
		return fmt.Errorf("failed to create file (%s):\n%w", outputInitrdImagePath, err)
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

	err = filepath.Walk(inputDir, func(path string, info os.FileInfo, fileErr error) (err error) {
		if fileErr != nil {
			logger.Log.Warnf("File walk error on path (%s), error: %s", path, fileErr)
			return fileErr
		}
		err = addFileToArchive(inputDir, path, info, cpioWriter)
		if err != nil {
			logger.Log.Warnf("Failed to add (%s), error: %s", path, err)
		}
		return nil
	})

	return nil
}

func addFileToArchive(inputDir, path string, info os.FileInfo, cpioWriter *cpio.Writer) (err error) {
	// Get the relative path of the file compared to the input directory.
	// The input directory should be considered the "root" of the cpio archive.
	relPath, err := filepath.Rel(inputDir, path)
	if err != nil {
		return
	}

	// logger.Log.Debugf("Adding to initrd: %s", relPath)

	// Symlinks need to be resolved to their target file to be added to the cpio archive.
	var link string
	if info.Mode()&os.ModeSymlink != 0 {
		link, err = os.Readlink(path)
		if err != nil {
			return
		}

		// logger.Log.Debugf("--> Adding link: (%s) -> (%s)", relPath, link)
	}

	// Convert the OS header into a CPIO header
	header, err := cpio.FileInfoHeader(info, link)
	if err != nil {
		return
	}

	// Get owners
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("failed to change mode of file (0) %s: %w", path, err)
	}

	header.UID = int(stat.Uid)
	header.GID = int(stat.Gid)

	// The default OS header will only have the filename as "Name".
	// Manually set the CPIO header's Name field to the relative path so it
	// is extracted to the correct directory.
	header.Name = relPath

	err = cpioWriter.WriteHeader(header)
	if err != nil {
		return
	}

	// Special files (unix sockets, directories, symlinks, ...) need to be handled differently
	// since a simple byte transfer of the file's content into the CPIO archive can't be achieved.
	if !info.Mode().IsRegular() {
		// For a symlink the reported size will be the size (in bytes) of the link's target.
		// Write this data into the archive.
		if info.Mode()&os.ModeSymlink != 0 {
			_, err = cpioWriter.Write([]byte(link))
		}

		// For all other special files, they will be of size 0 and only contain the header in the archive.
		return
	}

	// For regular files, open the actual file and copy its content into the archive.
	fileToAdd, err := os.Open(path)
	if err != nil {
		return
	}
	defer fileToAdd.Close()

	_, err = io.Copy(cpioWriter, fileToAdd)
	return
}

func CreateFolderFromInitrdImage(inputInitrdImagePath, outputDir string) error {
	inputInitrdImageFile, err := os.Open(inputInitrdImagePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer inputInitrdImageFile.Close()

	// Create pgzip reader
	gzr, err := pgzip.NewReader(inputInitrdImageFile)
	if err != nil {
		return fmt.Errorf("create pgzip reader: %w", err)
	}
	defer gzr.Close()

	// Create cpio reader
	cpioReader := cpio.NewReader(gzr)

	for {
		hdr, err := cpioReader.Next()
		if err == io.EOF {
			break // end of archive
		}
		if err != nil {
			return fmt.Errorf("read cpio header: %w", err)
		}

		path := filepath.Join(outputDir, hdr.Name)
		fileMode := os.FileMode(hdr.Mode & (cpio.ModePerm | cpio.ModeSetuid | cpio.ModeSetgid | cpio.ModeSticky))
		fileType := hdr.Mode & cpio.ModeType

		switch fileType {
		case cpio.ModeDir:
			err := os.MkdirAll(path, fileMode)
			if err != nil {
				return fmt.Errorf("failed to create directory %s: %w", path, err)
			}

			if hdr.UID != 0 || hdr.GID != 0 {
				logger.Log.Infof("---- unpacking ---- %d, %d, %s", hdr.UID, hdr.GID, path)
				// time.Sleep(20 * time.Second)
			}

			err = os.Chown(path, hdr.UID, hdr.GID)
			if err != nil {
				return fmt.Errorf("failed to set ownership on extracted file (%s) to (%d,%d):\n%w", path, hdr.UID, hdr.GID, err)
			}

			if fileMode > 0777 {
				// For some reason, Chmod does not set the higher-order bits:
				// setuid, setgid, and setsticky bits
				// err = os.Chmod(path, fileMode)
				fileModeString := fmt.Sprintf("%#o", fileMode)
				chmodParams := []string{
					fileModeString,
					path,
				}
				err = shell.ExecuteLive(true /*squashErrors*/, "chmod", chmodParams...)
				if err != nil {
					return fmt.Errorf("failed to change mode of file %s: %w", path, err)
				}
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

			err = os.Chown(path, hdr.UID, hdr.GID)
			if err != nil {
				return fmt.Errorf("failed to set ownership on extracted file (%s) to (%d,%d):\n%w", path, hdr.UID, hdr.GID, err)
			}

			if fileMode > 0777 {
				// For some reason, Chmod does not set the higher-order bits:
				// setuid, setgid, and setsticky bits
				// err = os.Chmod(path, fileMode)
				fileModeString := fmt.Sprintf("%#o", fileMode)
				chmodParams := []string{
					fileModeString,
					path,
				}
				err = shell.ExecuteLive(true /*squashErrors*/, "chmod", chmodParams...)
				if err != nil {
					return fmt.Errorf("failed to change mode of file %s: %w", path, err)
				}
			}

		case cpio.ModeSymlink:
			pathDir := filepath.Dir(path)
			_, err := os.Stat(pathDir)
			if err != nil {
				if os.IsNotExist(err) {
					logger.Log.Debugf("                 --> Directory (%s) does not exists!", pathDir)
					// ToDo: why 755?
					err := os.MkdirAll(pathDir, 0755)
					if err != nil {
						return fmt.Errorf("failed to create directory %s: %w", path, err)
					}
				} else {
					return fmt.Errorf("failed to check directory %s: %w", pathDir, err)
				}
			}

			os.Symlink(hdr.Linkname, path)
		case cpio.ModeDevice:
			logger.Log.Debugf("-- [dev     ] [%#o] (%s)", fileMode, path)
			return fmt.Errorf("unsupported file type 'Device' in CPIO archive.")
		case cpio.ModeCharDevice:
			logger.Log.Debugf("-- [char dev] [%#o] (%s)", fileMode, path)
			return fmt.Errorf("unsupported file type 'Char Device' in CPIO archive.")
		case cpio.ModeSocket:
			logger.Log.Debugf("-- [socket  ] [%#o] (%s)", fileMode, path)
			return fmt.Errorf("unsupported file type 'Socket' in CPIO archive.")
		default:
			return fmt.Errorf("unsupported unknown type %#o in CPIO archive.", fileType)
		}
	}

	return nil
}
