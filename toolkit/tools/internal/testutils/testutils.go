package testutils

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/buildpipeline"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/pkg/imageconnection"
	"github.com/stretchr/testify/assert"
)

const (
	testImageRootDirName = "testimageroot"
)

type MountPoint struct {
	PartitionNum   int
	Path           string
	FileSystemType string
	Flags          uintptr
}

func GetImageFileType(filePath string) (string, error) {
	file, err := os.OpenFile(filePath, os.O_RDONLY, 0)
	if err != nil {
		return "", err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", err
	}

	firstBytes := make([]byte, 512)
	firstBytesCount, err := file.Read(firstBytes)
	if err != nil {
		return "", err
	}

	lastBytes := make([]byte, 512)
	lastBytesCount, err := file.ReadAt(lastBytes, max(0, stat.Size()-512))
	if err != nil {
		return "", err
	}

	switch {
	case firstBytesCount >= 8 && bytes.Equal(firstBytes[:8], []byte("conectix")):
		return "vhd", nil

	case firstBytesCount >= 8 && bytes.Equal(firstBytes[:8], []byte("vhdxfile")):
		return "vhdx", nil

	case isZstFile(firstBytes):
		return "zst", nil

	// Check for the MBR signature (which exists even on GPT formatted drives).
	case firstBytesCount >= 512 && bytes.Equal(firstBytes[510:512], []byte{0x55, 0xAA}):
		switch {
		case lastBytesCount >= 512 && bytes.Equal(lastBytes[:8], []byte("conectix")):
			return "vhd-fixed", nil

		default:
			return "raw", nil
		}

	default:
		return "", fmt.Errorf("unknown file type: %s", filePath)
	}
}

func isZstFile(firstBytes []byte) bool {
	if len(firstBytes) < 4 {
		return false
	}

	magicNumber := binary.LittleEndian.Uint32(firstBytes[:4])

	// 0xFD2FB528 is a zst frame.
	// 0x184D2A50-0x184D2A5F are skippable ztd frames.
	return magicNumber == 0xFD2FB528 || (magicNumber >= 0x184D2A50 && magicNumber <= 0x184D2A5F)
}

func GetDownloadedRpmsDir(t *testing.T, testutilsDir string, azureLinuxVersion string, imagecreator bool) string {
	downloadedRpmsDir := filepath.Join(testutilsDir, "testrpms/downloadedrpms", azureLinuxVersion)
	dirExists, err := file.DirExists(downloadedRpmsDir)
	if !assert.NoErrorf(t, err, "cannot access downloaded RPMs dir (%s)", downloadedRpmsDir) {
		t.FailNow()
	}
	if !assert.True(t, dirExists) {
		// log the downloadedRpmsDir
		t.Logf("downloadedRpmsDir: %s", downloadedRpmsDir)
		t.Logf("test requires offline RPMs")
		t.Logf("please run toolkit/tools/internal/testutils/testrpms/download-test-utils.sh -t %s -s %t",
			azureLinuxVersion, imagecreator)
		t.FailNow()
	}

	return downloadedRpmsDir
}

func GetDownloadedToolsFile(t *testing.T, testutilsDir string, azureLinuxVersion string, imagecreator bool) string {
	GetDownloadedToolsFile := filepath.Join(testutilsDir, "testrpms/tools.tar.gz")
	if !assert.FileExists(t, GetDownloadedToolsFile) {
		t.Logf("test requires downloaded tools file")
		t.Logf("please run toolkit/tools/internal/testutils/testrpms/download-test-utils.sh -t %s -s %t",
			azureLinuxVersion, imagecreator)
		t.FailNow()
	}
	return GetDownloadedToolsFile
}

func GetDownloadedRpmsRepoFile(t *testing.T, testutilsDir string, azureLinuxVersion string, withGpgKey bool, imagecreator bool) string {
	dir := GetDownloadedRpmsDir(t, testutilsDir, azureLinuxVersion, imagecreator)

	suffix := "nokey"
	if withGpgKey {
		suffix = "withkey"
	}

	repoFile := filepath.Join(dir, "../", fmt.Sprintf("rpms-%s-%s.repo", azureLinuxVersion, suffix))
	return repoFile
}

func CheckSkipForCustomizeImageRequirements(t *testing.T) {
	if !buildpipeline.IsRegularBuild() {
		t.Skip("loopback block device not available")
	}

	if os.Geteuid() != 0 {
		t.Skip("Test must be run as root because it uses a chroot")
	}
}

func ConnectToImage(buildDir string, imageFilePath string, includeDefaultMounts bool, mounts []MountPoint,
) (*imageconnection.ImageConnection, error) {
	imageConnection := imageconnection.NewImageConnection()
	err := imageConnection.ConnectLoopback(imageFilePath)
	if err != nil {
		imageConnection.Close()
		return nil, err
	}

	rootDir := filepath.Join(buildDir, testImageRootDirName)

	mountPoints := []*safechroot.MountPoint(nil)
	for _, mount := range mounts {
		devPath := PartitionDevPath(imageConnection, mount.PartitionNum)

		var mountPoint *safechroot.MountPoint
		if mount.Path == "/" {
			mountPoint = safechroot.NewPreDefaultsMountPoint(devPath, mount.Path, mount.FileSystemType, mount.Flags,
				"")
		} else {
			mountPoint = safechroot.NewMountPoint(devPath, mount.Path, mount.FileSystemType, mount.Flags, "")
		}

		mountPoints = append(mountPoints, mountPoint)
	}

	err = imageConnection.ConnectChroot(rootDir, false, []string{}, mountPoints, includeDefaultMounts)
	if err != nil {
		imageConnection.Close()
		return nil, err
	}

	return imageConnection, nil
}

func PartitionDevPath(imageConnection *imageconnection.ImageConnection, partitionNum int) string {
	devPath := fmt.Sprintf("%sp%d", imageConnection.Loopback().DevicePath(), partitionNum)
	return devPath
}
