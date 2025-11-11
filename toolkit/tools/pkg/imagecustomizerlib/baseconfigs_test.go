package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/systemd"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/userutils"
	"github.com/stretchr/testify/assert"
)

func TestBaseConfigsInputAndOutput(t *testing.T) {
	testTempDir := filepath.Join(tmpDir, "TestBaseConfigsInputAndOutput")
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	currentConfigFile := filepath.Join(testDir, "hierarchical-config.yaml")

	options := ImageCustomizerOptions{
		BuildDir:             buildDir,
		UseBaseImageRpmRepos: true,
	}

	var config imagecustomizerapi.Config
	err := imagecustomizerapi.UnmarshalYamlFile(currentConfigFile, &config)
	assert.NoError(t, err)

	rc, err := ValidateConfig(t.Context(), testDir, &config, false, options)
	assert.NoError(t, err)

	// Verify resolved values
	expectedInputPath := file.GetAbsPathWithBase(testDir, "testimages/empty.vhdx")
	expectedOutputPath := file.GetAbsPathWithBase(testDir, "./out/output-image-2.vhdx")
	expectedArtifactsPath := file.GetAbsPathWithBase(testDir, "./artifacts-2")

	assert.Equal(t, expectedInputPath, rc.InputImage.Path)
	assert.Equal(t, expectedOutputPath, rc.OutputImageFile)
	assert.Equal(t, expectedArtifactsPath, rc.OutputArtifacts.Path)
	assert.Equal(t, "test-hostname", rc.Hostname)

	// Verify merged artifact items
	expectedItems := []imagecustomizerapi.OutputArtifactsItemType{
		imagecustomizerapi.OutputArtifactsItemUkis,
		imagecustomizerapi.OutputArtifactsItemShim,
	}
	actual := rc.OutputArtifacts.Items
	assert.Equal(t, len(expectedItems), len(actual))

	assert.ElementsMatch(t, expectedItems, actual,
		"output artifact items should match - expected: %v, got: %v", expectedItems, actual)
}

func TestBaseConfigsFullRun(t *testing.T) {
	baseImage, baseImageInfo := checkSkipForCustomizeDefaultImage(t)
	if baseImageInfo.Version == baseImageVersionAzl2 {
		t.Skip("'systemd-boot' is not available on Azure Linux 2.0")
	}

	ukifyExists, err := file.CommandExists("ukify")
	assert.NoError(t, err)
	if !ukifyExists {
		t.Skip("The 'ukify' command is not available")
	}

	if runtime.GOARCH == "arm64" {
		t.Skip("systemd-boot not available on AZL3 ARM64 yet")
	}

	testTmpDir := filepath.Join(tmpDir, "TestBaseConfigsFullRun")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	currentConfigFile := filepath.Join(testDir, "hierarchical-config.yaml")

	err = CustomizeImageWithConfigFile(t.Context(), buildDir, currentConfigFile, baseImage, nil,
		outImageFilePath, "raw", true, "")
	if !assert.NoError(t, err) {
		return
	}

	assert.FileExists(t, outImageFilePath)

	mountPoints := []testutils.MountPoint{
		{
			PartitionNum:   3,
			Path:           "/",
			FileSystemType: "ext4",
		},
		{
			PartitionNum:   2,
			Path:           "/boot",
			FileSystemType: "ext4",
		},
		{
			PartitionNum:   1,
			Path:           "/boot/efi",
			FileSystemType: "vfat",
		},
	}

	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, true /*includeDefaultMounts*/, mountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Verify hostname
	actualHostname, err := os.ReadFile(filepath.Join(imageConnection.Chroot().RootDir(), "etc/hostname"))
	assert.NoError(t, err)
	assert.Equal(t, "test-hostname", string(actualHostname))

	// Verify users
	baseadminEntry, err := userutils.GetPasswdFileEntryForUser(imageConnection.Chroot().RootDir(), "test-user-base")
	if assert.NoError(t, err) {
		assert.Contains(t, baseadminEntry.HomeDirectory, "test-user-base")
	}

	currentuserEntry, err := userutils.GetPasswdFileEntryForUser(imageConnection.Chroot().RootDir(), "test-user")
	if assert.NoError(t, err) {
		assert.Contains(t, currentuserEntry.HomeDirectory, "test-user")
	}

	// Verify groups
	_, err = userutils.GetGroupEntry(imageConnection.Chroot().RootDir(), "test-group-base")
	assert.NoError(t, err)

	_, err = userutils.GetGroupEntry(imageConnection.Chroot().RootDir(), "test-group")
	assert.NoError(t, err)

	// Verify additional files
	aFilePath := filepath.Join(imageConnection.Chroot().RootDir(), "mnt/a/a.txt")
	bFilePath := filepath.Join(imageConnection.Chroot().RootDir(), "mnt/b/b.txt")

	_, err = os.Stat(aFilePath)
	assert.NoError(t, err, "expected a.txt to exist at %s", aFilePath)
	_, err = os.Stat(bFilePath)
	assert.NoError(t, err, "expected b.txt to exist at %s", bFilePath)

	// Verify additional dirs
	animalsFileOrigPath := filepath.Join(testDir, "dirs/a/usr/local/bin/animals.sh")
	animalsFileNewPath := filepath.Join(imageConnection.Chroot().RootDir(), "/usr/local/bin/animals.sh")

	verifyFileContentsSame(t, animalsFileOrigPath, animalsFileNewPath)

	plantsFileOrigPath := filepath.Join(testDir, "dirs/a/usr/local/bin/plants.sh")
	plantsFileNewPath := filepath.Join(imageConnection.Chroot().RootDir(), "/usr/local/bin/plants.sh")

	verifyFileContentsSame(t, plantsFileOrigPath, plantsFileNewPath)

	// Verify packages
	nginxInstalled := isPackageInstalled(imageConnection.Chroot(), "nginx")
	assert.True(t, nginxInstalled)

	nginxVersionOutput, err := getPkgVersionFromChroot(imageConnection, "nginx")
	assert.NoError(t, err, "failed to retrieve nginx version from chroot")

	nginxExpectedVersion := "nginx-1.25.4-5"
	assert.Containsf(t, nginxVersionOutput, nginxExpectedVersion,
		"should install nginx version %s, but got: %s", nginxExpectedVersion, nginxVersionOutput)

	sshdInstalled := isPackageInstalled(imageConnection.Chroot(), "openssh-server")
	assert.True(t, sshdInstalled)

	systemdBootVersionOutput, err := getPkgVersionFromChroot(imageConnection, "systemd-boot")
	assert.NoError(t, err, "failed to retrieve systemd-boot version from chroot")

	systemdBootExpectedVersion := "systemd-boot-255-24"
	assert.Containsf(t, systemdBootVersionOutput, systemdBootExpectedVersion,
		"should install systemd-boot version %s, but got: %s", systemdBootExpectedVersion, systemdBootVersionOutput)

	// Verify services
	sshdEnabled, err := systemd.IsServiceEnabled("sshd", imageConnection.Chroot())
	assert.NoError(t, err)
	assert.True(t, sshdEnabled)

	nginxEnabled, err := systemd.IsServiceEnabled("nginx", imageConnection.Chroot())
	assert.NoError(t, err)
	assert.True(t, nginxEnabled)

	consoleGettyEnabled, err := systemd.IsServiceEnabled("console-getty", imageConnection.Chroot())
	assert.NoError(t, err)
	assert.False(t, consoleGettyEnabled)

	// Verify modules
	moduleLoadFilePath := filepath.Join(imageConnection.Chroot().RootDir(), moduleLoadPath)
	moduleDisableFilePath := filepath.Join(imageConnection.Chroot().RootDir(), moduleDisabledPath)

	moduleLoadContent, err := os.ReadFile(moduleLoadFilePath)
	if err != nil {
		t.Errorf("Failed to read module load configuration file: %v", err)
	}

	moduleDisableContent, err := os.ReadFile(moduleDisableFilePath)
	if err != nil {
		t.Errorf("Failed to read module disable configuration file: %v", err)
	}

	assert.Contains(t, string(moduleLoadContent), "br_netfilter")
	assert.Contains(t, string(moduleDisableContent), "vfio")

	// Verify SELinux
	verifyKernelCommandLine(t, imageConnection, []string{}, []string{"security=selinux", "selinux=1", "enforcing=1"})
	verifySELinuxConfigFile(t, imageConnection, "disabled")

	// Verify overlays
	fstabPath := filepath.Join(imageConnection.Chroot().RootDir(), "etc/fstab")
	fstabContents, err := file.Read(fstabPath)
	if !assert.NoError(t, err) {
		return
	}

	assert.Contains(t, fstabContents,
		"overlay /var overlay lowerdir=/var,upperdir=/mnt/overlays/var/upper,workdir=/mnt/overlays/var/work 0 0")

	assert.Contains(t, fstabContents,
		"overlay /etc overlay lowerdir=/etc,upperdir=/var/overlays/etc/upper,workdir=/var/overlays/etc/work 0 0")

	// Verify UKI creation
	ukiDir := filepath.Join(imageConnection.Chroot().RootDir(), "boot/efi/EFI/Linux")
	files, err := os.ReadDir(ukiDir)
	if err != nil {
		t.Errorf("Failed to read UKI directory: %v", err)
		return
	}
	var ukiFiles []string
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".efi") {
			ukiFiles = append(ukiFiles, f.Name())
		}
	}
	assert.Len(t, ukiFiles, 1, "expected one UKI .efi file to be created")

	// Verify kernel commandline
	grubCfgFilePath := filepath.Join(imageConnection.Chroot().RootDir(), "/boot/grub2/grub.cfg")
	grubCfgContents, err := file.Read(grubCfgFilePath)
	assert.NoError(t, err)
	assert.NotContains(t, grubCfgContents, "rd.info")
	assert.Contains(t, grubCfgContents, "console=tty0 console=ttyS0")
}
