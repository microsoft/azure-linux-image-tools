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
		err = addFileToArchive(inputDir, path, info, cpioWriter)
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

	// Get owners (cpio.FileInfoHeader() does not fill out this part)
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("failed to get file stat of (%s)", path)
	}
	cpioHeader.UID = int(stat.Uid)
	cpioHeader.GID = int(stat.Gid)

	// Convert full path to relative path
	relPath, err := filepath.Rel(inputDir, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get relative path of (%s) using root (%s):\n%w", path, inputDir, err)
	}
	cpioHeader.Name = relPath

	return cpioHeader, nil
}

func addFileToArchive(inputDir, path string, info os.FileInfo, cpioWriter *cpio.Writer) (err error) {
	// Symlinks need to be resolved to their target file to be added to the cpio archive.
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
		// For a symlink, the reported size will be the size (in bytes) of the link's target.
		// Write this data into the archive.
		if info.Mode()&os.ModeSymlink != 0 {
			_, err = cpioWriter.Write([]byte(link))
			if err != nil {
				return fmt.Errorf("failed to write link (%s)\n%w", path, err)
			}
		}

		// For all other special files, they will be of size 0 and only contain the header in the archive.
	}

	return nil
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
