package testutils

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

const (
	testImageRootDirName = "testimageroot"
)

type MountPoint struct {
	PartitionNum   int
	Path           string
	FileSystemType string
	Flags          uintptr
	Data           string
	// When set, use a read-only mapping owned by the image connection.
	Verity *VerityMount
}

type VerityMount struct {
	HashPartitionNum int
	// Supplied by the caller; the verity superblock does not contain the root hash.
	RootHash   string
	HashOffset uint64
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

	case firstBytesCount >= 4 && bytes.Equal(firstBytes[:4], []byte{'Q', 'F', 'I', 0xfb}):
		return "qcow2", nil

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

func GetDownloadedRpmsDir(t *testing.T, testutilsDir string, distro string, distroVersion string,
) string {
	if distro == "ubuntu" {
		t.Skip("test requires downloaded RPMs dir:\nRPMs dir not supported for Ubuntu yet")
	}

	downloadedRpmsDir := filepath.Join(testutilsDir, "testrpms/downloadedrpms", distro, distroVersion)
	if _, err := os.Stat(downloadedRpmsDir); os.IsNotExist(err) {
		t.Skipf("test requires downloaded RPMs dir: %s;\n"+
			"please run toolkit/tools/internal/testutils/testrpms/download-test-utils.sh -d %s -t %s",
			downloadedRpmsDir, distro, distroVersion)
	}

	return downloadedRpmsDir
}

func GetDownloadedToolsDir(t *testing.T, testutilsDir string, distro string, distroVersion string,
) string {
	if distro == "ubuntu" {
		t.Skip("test requires downloaded tools dir:\ntools dir not supported for Ubuntu yet")
	}

	toolsDirName := fmt.Sprintf("tools-%s-%s-dir", distro, distroVersion)
	toolsDirPath := filepath.Join(testutilsDir, "testrpms/build", toolsDirName)

	if _, err := os.Stat(toolsDirPath); os.IsNotExist(err) {
		t.Skipf("test requires downloaded tools dir: %s;\n"+
			"please run toolkit/tools/internal/testutils/testrpms/download-test-utils.sh -d %s -t %s",
			toolsDirPath, distro, distroVersion)
	}

	return toolsDirPath
}

func GetDownloadedRpmsRepoFile(t *testing.T, testutilsDir string, distro string, distroVersion string, withGpgKey bool,
) string {
	dir := GetDownloadedRpmsDir(t, testutilsDir, distro, distroVersion)

	suffix := "nokey"
	if withGpgKey {
		suffix = "withkey"
	}

	repoFile := filepath.Join(dir, "../../", fmt.Sprintf("rpms-%s-%s-%s.repo", distro, distroVersion, suffix))
	return repoFile
}

func CheckSkipForCustomizeImageRequirements(t *testing.T) {
	if testing.Short() {
		t.Skip("Short mode enabled")
	}

	if os.Geteuid() != 0 {
		t.Skip("Test must be run as root because it uses a chroot")
	}

	qemuimgExists, err := file.CommandExists("qemu-img")
	assert.NoError(t, err)
	if !qemuimgExists {
		t.Skip("The 'qemu-img' command is not available")
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
	verityDevices := make(map[verityMountKey]string)
	for _, mount := range mounts {
		devPath := PartitionDevPath(imageConnection, mount.PartitionNum)
		flags := mount.Flags
		if mount.Verity != nil {
			devPath, err = connectVerity(imageConnection, mount.PartitionNum, *mount.Verity, verityDevices)
			if err != nil {
				imageConnection.Close()
				return nil, err
			}
			flags |= unix.MS_RDONLY
		}

		var mountPoint *safechroot.MountPoint
		if mount.Path == "/" {
			mountPoint = safechroot.NewPreDefaultsMountPoint(devPath, mount.Path, mount.FileSystemType, flags,
				mount.Data)
		} else {
			mountPoint = safechroot.NewMountPoint(devPath, mount.Path, mount.FileSystemType, flags, mount.Data)
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

type verityMountKey struct {
	PartitionNum int
	VerityMount
}

func connectVerity(connection *imageconnection.ImageConnection, partitionNum int, verity VerityMount,
	devices map[verityMountKey]string,
) (string, error) {
	if verity.RootHash == "" {
		return "", fmt.Errorf("verity mount for partition (%d) requires a root hash", partitionNum)
	}
	key := verityMountKey{PartitionNum: partitionNum, VerityMount: verity}
	if path, exists := devices[key]; exists {
		return path, nil
	}
	name := fmt.Sprintf("ic-test-verity-%s-%d", filepath.Base(connection.Loopback().DevicePath()), len(devices))
	dataDevice := PartitionDevPath(connection, partitionNum)
	hashDevice := PartitionDevPath(connection, verity.HashPartitionNum)
	err := shell.NewExecBuilder("veritysetup", "open", fmt.Sprintf("--hash-offset=%d", verity.HashOffset),
		dataDevice, name, hashDevice, verity.RootHash).Execute()
	if err != nil {
		return "", fmt.Errorf("failed to open verity device for partition (%d):\n%w", partitionNum, err)
	}
	connection.AddOwnedCleanup(func() error {
		return shell.NewExecBuilder("veritysetup", "close", name).Execute()
	})
	path := filepath.Join("/dev/mapper", name)
	devices[key] = path
	return path, nil
}
