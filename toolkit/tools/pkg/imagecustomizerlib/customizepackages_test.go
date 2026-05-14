// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/sliceutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestCustomizeImagePackagesAddOfflineDir(t *testing.T) {
	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImagePackagesAddOfflineDir")
	defer os.RemoveAll(testTmpDir)

	baseImage, baseImageInfo := checkSkipForCustomizeDefaultAzureLinuxImage(t)
	downloadedRpmsDir := testutils.GetDownloadedRpmsDir(t, testutilsDir, baseImageInfo.Distro, baseImageInfo.Version,
		false)
	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	downloadedRpmsTmpDir := filepath.Join(testTmpDir, "rpms")

	// Create a copy of the RPMs directory, but without the tree package.
	err := copyRpms(downloadedRpmsDir, downloadedRpmsTmpDir, []string{"tree-"})
	if !assert.NoError(t, err) {
		return
	}

	packageName := "unzip"
	packagePath := "/usr/bin/unzip"
	if baseImageInfo.Distro == baseImageDistroAzureLinux && baseImageInfo.Version == "4.0" {
		packageName = "jq"
		packagePath = "/usr/bin/jq"
	}

	// Install package.
	config := imagecustomizerapi.Config{
		OS: &imagecustomizerapi.OS{
			Packages: imagecustomizerapi.Packages{
				Install: []string{packageName},
			},
		},
	}

	err = CustomizeImage(t.Context(), buildDir, testDir, &config, baseImage, []string{downloadedRpmsTmpDir}, outImageFilePath,
		"raw", false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err := connectToAzureLinuxCoreEfiImage(buildDir, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Ensure package was installed.
	ensureFilesExist(t, imageConnection,
		packagePath,
	)

	// Ensure tree was not installed.
	ensureFilesNotExist(t, imageConnection,
		"/usr/bin/tree",
	)

	verifyImageHistoryFile(t, 1, config, imageConnection.Chroot().RootDir())

	err = imageConnection.CleanClose()
	if !assert.NoError(t, err) {
		return
	}

	// Create a copy of the RPMs directory, but without the  package.
	// This ensures that the package repo metadata is refreshed between runs.
	err = os.RemoveAll(downloadedRpmsTmpDir)
	if !assert.NoError(t, err) {
		return
	}

	err = copyRpms(downloadedRpmsDir, downloadedRpmsTmpDir, []string{packageName + "-"})
	if !assert.NoError(t, err) {
		return
	}

	// Install tree package.
	config = imagecustomizerapi.Config{
		OS: &imagecustomizerapi.OS{
			Packages: imagecustomizerapi.Packages{
				InstallLists: []string{"lists/tree.yaml"},
			},
		},
	}

	err = CustomizeImage(t.Context(), buildDir, testDir, &config, outImageFilePath, []string{downloadedRpmsTmpDir}, outImageFilePath,
		"raw", false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err = connectToAzureLinuxCoreEfiImage(buildDir, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Ensure the previously installed package and tree was installed.
	ensureFilesExist(t, imageConnection,
		packagePath,
		"/usr/bin/tree",
	)

	verifyImageHistoryFile(t, 2, config, imageConnection.Chroot().RootDir())
}

func copyRpms(sourceDir string, targetDir string, excludePrefixes []string) error {
	sourceFiles, err := os.ReadDir(sourceDir)
	if err != nil {
		return fmt.Errorf("failed to read source directory (%s):\n%w", sourceDir, err)
	}

	for _, sourceFile := range sourceFiles {
		// Skip anything that isn't a regular file.
		// The downloaded RPMs directory may also contain GPG key symlinks, some of which intentionally dangle, such as
		// the Azure Linux 4.0 evergreen-* -> 5.0-primary chain.
		if !sourceFile.Type().IsRegular() {
			continue
		}

		exclude := sliceutils.ContainsFunc(excludePrefixes, func(prefix string) bool {
			return strings.HasPrefix(sourceFile.Name(), prefix)
		})
		if exclude {
			continue
		}

		err := file.Copy(filepath.Join(sourceDir, sourceFile.Name()), filepath.Join(targetDir, sourceFile.Name()))
		if err != nil {
			return err
		}
	}

	return nil
}

func TestCustomizeImagePackagesAddOfflineLocalRepoWithGpgKey(t *testing.T) {
	testCustomizeImagePackagesAddOfflineLocalRepoHelper(t, "TestCustomizeImagePackagesAddOfflineLocalRepoWithGpgKey",
		true)
}

func TestCustomizeImagePackagesAddOfflineLocalRepoNoGpgKey(t *testing.T) {
	testCustomizeImagePackagesAddOfflineLocalRepoHelper(t, "TestCustomizeImagePackagesAddOfflineLocalRepoNoGpgKey",
		false)
}

func testCustomizeImagePackagesAddOfflineLocalRepoHelper(t *testing.T, testName string, withGpgKey bool) {
	testTmpDir := filepath.Join(tmpDir, testName)
	defer os.RemoveAll(testTmpDir)

	baseImage, baseImageInfo := checkSkipForCustomizeDefaultAzureLinuxImage(t)

	if baseImageInfo.Version == baseImageVersionAzl4 && withGpgKey {
		t.Skip("Azure Linux 4.0 currently ships with unsigned RPMs, cannot test with GPG key")
	}

	downloadedRpmsRepoFile := testutils.GetDownloadedRpmsRepoFile(t, testutilsDir, baseImageInfo.Distro,
		baseImageInfo.Version, withGpgKey, false)
	rpmSources := []string{downloadedRpmsRepoFile}

	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	configFile := filepath.Join(testDir, packagesAddConfigFile(t, baseImageInfo))

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, rpmSources, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err := connectToAzureLinuxCoreEfiImage(buildDir, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	packagePath := "/usr/bin/unzip"
	if baseImageInfo.Distro == baseImageDistroAzureLinux && baseImageInfo.Version == "4.0" {
		packagePath = "/usr/bin/jq"
	}

	// Ensure packages were installed.
	ensureFilesExist(t, imageConnection,
		packagePath,
		"/usr/bin/tree",
	)
}

// packagesAddConfigFile returns the packages-add-config test config file appropriate for the
// given base image version (azl3 vs azl4) and host architecture.
func packagesAddConfigFile(t *testing.T, baseImageInfo testBaseImageInfo) string {
	switch baseImageInfo.Version {
	case baseImageVersionAzl2, baseImageVersionAzl3, baseImageVersionUbuntu2204, baseImageVersionUbuntu2404:
		return "packages-add-config.yaml"
	case baseImageVersionAzl4:
		return "packages-add-config-azl4.yaml"
	default:
		t.Fatalf("unsupported base image version for packages-add-config test: %s", baseImageInfo.Version)
		return ""
	}
}

func TestCustomizeImagePackagesUpdateAfterInstall(t *testing.T) {
	for _, baseImageInfo := range checkSkipForCustomizeDefaultImages(t) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImagePackagesUpdateAfterInstall(t, baseImageInfo)
		})
	}
}

func testCustomizeImagePackagesUpdateAfterInstall(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestCustomizeImagePackagesUpdateAfterInstall_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")

	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	configFile := filepath.Join(testDir, packagesUpdateConfigFile(t, baseImageInfo))

	err := CustomizeImageWithConfigFileOptions(t.Context(), configFile, ImageCustomizerOptions{
		BuildDir:             buildDir,
		InputImageFile:       baseImage,
		OutputImageFile:      outImageFilePath,
		OutputImageFormat:    "raw",
		UseBaseImageRpmRepos: true,
		PreviewFeatures:      baseImageInfo.PreviewFeatures,
	})
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, false, baseImageInfo.MountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	packagePath := "/usr/bin/unzip"
	if baseImageInfo.Distro == baseImageDistroAzureLinux && baseImageInfo.Version == "4.0" {
		packagePath = "/usr/bin/jq"
	}

	ensureFilesExist(t, imageConnection,
		packagePath,
	)

	ensurePackageCacheCleanup(t, imageConnection, baseImageInfo)
}

// packagesUpdateConfigFile returns the packages-update-config test config file appropriate for the
// given base image distro and version.
func packagesUpdateConfigFile(t *testing.T, baseImageInfo testBaseImageInfo) string {
	switch baseImageInfo.Distro {
	case baseImageDistroAzureLinux:
		switch baseImageInfo.Version {
		case baseImageVersionAzl2, baseImageVersionAzl3:
			return "packages-update-config.yaml"
		case baseImageVersionAzl4:
			return "packages-update-config-azl4.yaml"
		default:
			t.Fatalf("unsupported Azure Linux version for packages-update-config test: %s", baseImageInfo.Version)
			return ""
		}
	case baseImageDistroUbuntu:
		return "packages-update-config.yaml"
	default:
		t.Fatalf("unsupported distro for packages-update-config test: %s", baseImageInfo.Distro)
		return ""
	}
}

func TestCustomizeImagePackagesUpdateExisting(t *testing.T) {
	for _, baseImageInfo := range checkSkipForCustomizeDefaultImages(t) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImagePackagesUpdateExisting(t, baseImageInfo)
		})
	}
}

func testCustomizeImagePackagesUpdateExisting(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	if baseImageInfo.Distro == baseImageDistroUbuntu {
		t.Skip("Skipping Ubuntu since test fails due to disk space contraints")
	}

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestCustomizeImagePackagesUpdateExisting_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")

	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	configFile := filepath.Join(testDir, "packages-update-existing-config.yaml")

	err := CustomizeImageWithConfigFileOptions(t.Context(), configFile, ImageCustomizerOptions{
		BuildDir:             buildDir,
		InputImageFile:       baseImage,
		OutputImageFile:      outImageFilePath,
		OutputImageFormat:    "raw",
		UseBaseImageRpmRepos: true,
		PreviewFeatures:      baseImageInfo.PreviewFeatures,
	})
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, false, baseImageInfo.MountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	ensurePackageCacheCleanup(t, imageConnection, baseImageInfo)
}

func TestCustomizeImagePackagesRemove(t *testing.T) {
	for _, baseImageInfo := range checkSkipForCustomizeDefaultImages(t) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImagePackagesRemove(t, baseImageInfo)
		})
	}
}

func testCustomizeImagePackagesRemove(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestCustomizeImagePackagesRemove_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")

	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	removedInlineBinary := "/usr/sbin/chronyd"

	configFile := filepath.Join(testDir, "packages-remove-config.yaml")
	removedListBinary := "/usr/bin/curl"
	if baseImageInfo.Distro == baseImageDistroAzureLinux && baseImageInfo.Version == baseImageVersionAzl4 {
		configFile = filepath.Join(testDir, "packages-remove-config-azl4.yaml")
		removedListBinary = "/usr/bin/nano"
	}

	err := CustomizeImageWithConfigFileOptions(t.Context(), configFile, ImageCustomizerOptions{
		BuildDir:             buildDir,
		InputImageFile:       baseImage,
		OutputImageFile:      outImageFilePath,
		OutputImageFormat:    "raw",
		UseBaseImageRpmRepos: true,
		PreviewFeatures:      baseImageInfo.PreviewFeatures,
	})
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, false, baseImageInfo.MountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Verify both inline (chrony) and list-referenced (curl/nano) packages were removed.
	ensureFilesNotExist(t, imageConnection,
		removedInlineBinary,
		removedListBinary,
	)

	ensurePackageCacheCleanup(t, imageConnection, baseImageInfo)
}

// ensurePackageCacheCleanup verifies that package cache artifacts have been properly cleaned for the given distro.
func ensurePackageCacheCleanup(t *testing.T, imageConnection *imageconnection.ImageConnection,
	baseImageInfo testBaseImageInfo,
) {
	t.Helper()
	switch baseImageInfo.Distro {
	case baseImageDistroUbuntu:
		ensureAptCacheCleanup(t, imageConnection)
		ensureAptServicePreventionRestored(t, imageConnection)
	case baseImageDistroAzureLinux:
		switch baseImageInfo.Version {
		case baseImageVersionAzl2, baseImageVersionAzl3:
			ensureRpmCacheCleanup(t, imageConnection, "/var/cache/tdnf", baseImageInfo)
		case baseImageVersionAzl4:
			// Azure Linux 4 uses DNF 5, which stores its cache under /var/cache/libdnf5
			// (DNF 4's /var/cache/dnf is not present).
			ensureRpmCacheCleanup(t, imageConnection, "/var/cache/libdnf5", baseImageInfo)
		default:
			t.Fatalf("unsupported Azure Linux version for cache cleanup: %s", baseImageInfo.Version)
		}
	default:
		t.Fatalf("unsupported distro for cache cleanup: %s", baseImageInfo.Distro)
	}
}

// ensureAptCacheCleanup verifies that APT cache artifacts have been properly cleaned
func ensureAptCacheCleanup(t *testing.T, imageConnection *imageconnection.ImageConnection) {
	t.Helper()
	rootDir := imageConnection.Chroot().RootDir()

	// Verify no .deb files remain in the APT archive cache.
	archivesDir := filepath.Join(rootDir, "var/cache/apt/archives")
	archiveEntries, err := os.ReadDir(archivesDir)
	assert.NoError(t, err, "failed to read APT archives directory")

	var debFiles []string
	for _, entry := range archiveEntries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".deb" {
			debFiles = append(debFiles, entry.Name())
		}
	}
	assert.Empty(t, debFiles, "expected no .deb files in APT cache, but found: %v", debFiles)

	// Verify APT lists directory is empty (package metadata removed).
	aptListsDir := filepath.Join(rootDir, "var/lib/apt/lists")
	var listFiles []string
	err = filepath.WalkDir(aptListsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			listFiles = append(listFiles, path)
		}
		return nil
	})
	assert.NoError(t, err, "failed to walk APT lists directory")
	assert.Empty(t, listFiles, "expected no files in APT lists directory, but found %d files", len(listFiles))

	// Verify APT log files are truncated to zero bytes.
	aptLogDir := filepath.Join(rootDir, "var/log/apt")
	logEntries, err := os.ReadDir(aptLogDir)
	assert.NoError(t, err, "failed to read APT log directory: %s", aptLogDir)
	for _, entry := range logEntries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".log" {
			continue
		}

		fullPath := filepath.Join(aptLogDir, entry.Name())
		info, err := os.Stat(fullPath)
		assert.NoError(t, err, "failed to stat log file: %s", entry.Name())
		assert.Equal(t, int64(0), info.Size(),
			"expected APT log file %s to be truncated, but got %d bytes", entry.Name(), info.Size())
	}

	// Verify dpkg log is truncated to zero bytes (if it exists).
	dpkgLogPath := filepath.Join(rootDir, "var/log/dpkg.log")
	info, err := os.Stat(dpkgLogPath)
	if err == nil {
		assert.Equal(t, int64(0), info.Size(), "expected dpkg.log to be truncated (0 bytes), but got %d bytes", info.Size())
	} else {
		assert.ErrorIs(t, err, os.ErrNotExist, "unexpected error when checking dpkg.log")
	}
}

// ensureAptServicePreventionRestored verifies that APT service prevention files have been restored.
func ensureAptServicePreventionRestored(t *testing.T, imageConnection *imageconnection.ImageConnection) {
	ensureFilesNotExist(t, imageConnection,
		"/usr/sbin/policy-rc.d",
		"/sbin/start-stop-daemon.distrib",
	)
	ensureFilesExist(t, imageConnection,
		"/sbin/start-stop-daemon",
	)
}

func TestCustomizeImagePackagesDiskSpace(t *testing.T) {
	baseImage, baseImageInfo := checkSkipForCustomizeDefaultAzureLinuxImage(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImagePackagesDiskSpace")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")

	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	configFile := filepath.Join(testDir, installPackageDiskSpaceConfigFile(t, baseImageInfo))

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.ErrorContains(t, err, "failed to customize OS")
	assert.ErrorContains(t, err, "failed to install packages ([gcc])")
}

// installPackageDiskSpaceConfigFile returns the install-package-disk-space test config file appropriate for the
// given base image version (azl3 vs azl4) and host architecture.
func installPackageDiskSpaceConfigFile(t *testing.T, baseImageInfo testBaseImageInfo) string {
	switch baseImageInfo.Version {
	case baseImageVersionAzl2, baseImageVersionAzl3:
		return "install-package-disk-space-azl3.yaml"
	case baseImageVersionAzl4:
		return fmt.Sprintf("install-package-disk-space-%s-azl4.yaml", runtime.GOARCH)
	default:
		t.Fatalf("unsupported base image version for install-package-disk-space test: %s", baseImageInfo.Version)
		return ""
	}
}

func TestCustomizeImagePackagesUrlSource(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinuxCoreEfiAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImagePackagesUrlSourceHelper(t, baseImageInfo)
		})
	}
}

func testCustomizeImagePackagesUrlSourceHelper(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	var repoFileName string
	switch baseImageInfo.Version {
	case baseImageVersionAzl4:
		repoFileName = "cloud-native-azl4.repo"
	case baseImageVersionAzl3:
		repoFileName = "cloud-native-azl3.repo"
	default:
		assert.Fail(t, fmt.Sprintf("unexpected base image version: %s", baseImageInfo.Version))
	}

	repoFile := filepath.Join(testDir, "repos", repoFileName)
	if _, err := os.Stat(repoFile); os.IsNotExist(err) {
		t.Skipf("repo file not found: %s", repoFile)
	}

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestCustomizeImagePackagesUrlSource_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")

	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	configFile := filepath.Join(testDir, "packages-add-oras.yaml")

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, []string{repoFile}, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err := connectToAzureLinuxCoreEfiImage(buildDir, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Ensure packages were installed.
	ensureFilesExist(t, imageConnection,
		"/usr/bin/oras",
	)
}

func TestCustomizeImagePackagesBadRepo(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinuxCoreEfiAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImagePackagesBadRepoHelper(t, baseImageInfo)
		})
	}
}

func testCustomizeImagePackagesBadRepoHelper(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	if baseImageInfo.Version == baseImageVersionAzl4 {
		t.Skip("Azure Linux 4.0 does not yet support this test")
	}

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestCustomizeImagePackagesBadRepo_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")

	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	configFile := filepath.Join(testDir, "packages-add-oras.yaml")

	repoFile := filepath.Join(testDir, "repos/bad-repo.repo")

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, []string{repoFile}, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.ErrorIs(t, err, ErrPackageRepoMetadataRefresh)
}

func ensureRpmCacheCleanup(t *testing.T, imageConnection *imageconnection.ImageConnection, dirPath string,
	baseImageInfo testBaseImageInfo,
) {
	// Array to capture all the files of the provided root directory
	var existingFiles []string

	// Start the directory walk from the initial dirPath and collect all existing files
	fullPath := filepath.Join(imageConnection.Chroot().RootDir(), dirPath)
	if _, err := os.Stat(fullPath); errors.Is(err, fs.ErrNotExist) {
		return
	}

	err := filepath.WalkDir(fullPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("Failed to access path (%s):\n%w", path, err)
		}
		// Ignore files in the local-repo folder if the base image version is 2.0
		if !(strings.Contains(path, "local-repo") && baseImageInfo.Version == baseImageVersionAzl2) {
			fileInfo, err := os.Stat(path)
			if err != nil {
				return fmt.Errorf("failed to get file info for %s:\n%w", path, err)
			}
			if !fileInfo.IsDir() {
				// Append the file to the existingFiles array
				existingFiles = append(existingFiles, path)
			}
		}
		return nil
	})

	assert.NoError(t, err)

	// Ensure the cache has been cleaned up
	assert.Equal(t, 0, len(existingFiles), "Expected no file data in cache, but got %d files", len(existingFiles))
}

func TestCustomizeImagePackagesSnapshotTime(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinuxCoreEfiAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImagePackagesSnapshotTimeHelper(t, baseImageInfo)
		})
	}
}

func testCustomizeImagePackagesSnapshotTimeHelper(t *testing.T, baseImageInfo testBaseImageInfo) {
	if baseImageInfo.Version == baseImageVersionAzl4 {
		t.Skip("package snapshot time is not supported on Azure Linux 4")
	}

	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestCustomizeImagePackagesSnapshotTime_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	// Set the snapshot time to a date before unzip-6.0-22 (2025-04-16) was published, so unzip-6.0-21 is expected
	snapshotTime := "2025-01-01"

	config := imagecustomizerapi.Config{
		PreviewFeatures: []imagecustomizerapi.PreviewFeature{
			imagecustomizerapi.PreviewFeaturePackageSnapshotTime,
		},
		OS: &imagecustomizerapi.OS{
			Packages: imagecustomizerapi.Packages{
				Install:      []string{"unzip"},
				SnapshotTime: imagecustomizerapi.PackageSnapshotTime(snapshotTime),
			},
		},
	}

	err := CustomizeImage(t.Context(), buildDir, testDir, &config, baseImage, nil, outImageFilePath,
		"raw", true, "")
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, true, /*includeDefaultMounts*/
		azureLinuxCoreEfiMountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	ensureFilesExist(t, imageConnection,
		"/usr/bin/unzip",
	)

	unzipVersionOutput, err := getPkgVersionFromChroot(imageConnection, "unzip")
	assert.NoError(t, err, "failed to retrieve unzip version from chroot")

	expectedVersion := "unzip-6.0-21"
	assert.Containsf(t, unzipVersionOutput, expectedVersion,
		"snapshotTime %s should install unzip version %s, but got: %s", snapshotTime, expectedVersion, unzipVersionOutput)

	ensureFilesNotExist(t, imageConnection, customTdnfConfRelPath)

	verifyImageHistoryFile(t, 1, config, imageConnection.Chroot().RootDir())
}

func TestCustomizeImagePackagesCliSnapshotTimeOverridesConfigFile(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinuxCoreEfiAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImagePackagesCliSnapshotTimeOverridesConfigFileHelper(t, baseImageInfo)
		})
	}
}

func testCustomizeImagePackagesCliSnapshotTimeOverridesConfigFileHelper(t *testing.T, baseImageInfo testBaseImageInfo) {
	if baseImageInfo.Version == baseImageVersionAzl4 {
		t.Skip("package snapshot time is not supported on Azure Linux 4")
	}

	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestCustomizeImagePackagesCliSnapshotTimeOverridesConfigFile_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	snapshotTimeConfig := "2025-03-19"
	snapshotTimeCLI := "2025-01-01"

	config := imagecustomizerapi.Config{
		PreviewFeatures: []imagecustomizerapi.PreviewFeature{
			imagecustomizerapi.PreviewFeaturePackageSnapshotTime,
		},
		OS: &imagecustomizerapi.OS{
			Packages: imagecustomizerapi.Packages{
				Install:      []string{"unzip"},
				SnapshotTime: imagecustomizerapi.PackageSnapshotTime(snapshotTimeConfig),
			},
		},
	}

	// Set the snapshot time in CLI to a date before unzip-6.0-22 (2025-04-16) was published
	err := CustomizeImage(t.Context(), buildDir, testDir, &config, baseImage, nil, outImageFilePath,
		"raw", true, snapshotTimeCLI)
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, true, /*includeDefaultMounts*/
		azureLinuxCoreEfiMountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	ensureFilesExist(t, imageConnection,
		"/usr/bin/unzip",
	)

	unzipVersionOutput, err := getPkgVersionFromChroot(imageConnection, "unzip")
	assert.NoError(t, err, "failed to retrieve unzip version from chroot")

	expectedVersion := "unzip-6.0-21"
	assert.Containsf(t, unzipVersionOutput, expectedVersion,
		"snapshotTime %s should install unzip version %s, but got: %s", snapshotTimeCLI, expectedVersion, unzipVersionOutput)

	ensureFilesNotExist(t, imageConnection, customTdnfConfRelPath)

	verifyImageHistoryFile(t, 1, config, imageConnection.Chroot().RootDir())
}

func TestCustomizeImagePackagesSnapshotTimeWithoutPreviewFlagFails(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinuxCoreEfiAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImagePackagesSnapshotTimeWithoutPreviewFlagFailsHelper(t, baseImageInfo)
		})
	}
}

func testCustomizeImagePackagesSnapshotTimeWithoutPreviewFlagFailsHelper(t *testing.T, baseImageInfo testBaseImageInfo) {
	if baseImageInfo.Version == baseImageVersionAzl4 {
		t.Skip("package snapshot time is not supported on Azure Linux 4")
	}

	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestCustomizeImagePackagesSnapshotTimeWithoutPreviewFlagFails_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	config := imagecustomizerapi.Config{
		OS: &imagecustomizerapi.OS{
			Packages: imagecustomizerapi.Packages{
				Install:      []string{"unzip"},
				SnapshotTime: "2025-05-22",
			},
		},
	}

	err := CustomizeImage(t.Context(), buildDir, testDir, &config, baseImage, nil, outImageFilePath,
		"raw", true, "")
	assert.ErrorContains(t, err, "snapshotTime")
	assert.ErrorContains(t, err, "preview feature")
}

func TestCustomizeImagePackagesInstallOnline(t *testing.T) {
	for _, baseImageInfo := range checkSkipForCustomizeDefaultImages(t) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImagePackagesInstallOnline(t, baseImageInfo)
		})
	}
}

func testCustomizeImagePackagesInstallOnline(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestCustomizeImagePackagesInstallOnline_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	configFile := filepath.Join(testDir, packagesAddConfigFile(t, baseImageInfo))

	err := CustomizeImageWithConfigFileOptions(t.Context(), configFile, ImageCustomizerOptions{
		BuildDir:             buildDir,
		InputImageFile:       baseImage,
		OutputImageFile:      outImageFilePath,
		OutputImageFormat:    "raw",
		UseBaseImageRpmRepos: true, // Set to true for Azure Linux; it will be ignored for Ubuntu images.
		PreviewFeatures:      baseImageInfo.PreviewFeatures,
	})
	assert.NoError(t, err, "failed to customize image with config file options")

	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, false, baseImageInfo.MountPoints)
	assert.NoError(t, err, "failed to connect to image after customization")
	defer imageConnection.Close()

	packagePath := "/usr/bin/unzip"
	if baseImageInfo.Distro == baseImageDistroAzureLinux && baseImageInfo.Version == "4.0" {
		packagePath = "/usr/bin/jq"
	}

	// Verify both inline and list-referenced (tree) packages were installed.
	ensureFilesExist(t, imageConnection,
		packagePath,
		"/usr/bin/tree",
	)

	ensurePackageCacheCleanup(t, imageConnection, baseImageInfo)
}

func getPkgVersionFromChroot(imageConnection *imageconnection.ImageConnection, pkgName string) (string, error) {
	var versionOutput string
	out, _, err := shell.NewExecBuilder("rpm", "-q", pkgName).
		LogLevel(logrus.TraceLevel, logrus.DebugLevel).
		Chroot(imageConnection.Chroot().ChrootDir()).
		ExecuteCaptureOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get version of %s in chroot:\nfailed to query rpm:\n%w", pkgName, err)
	}

	versionOutput = strings.TrimSpace(out)
	return versionOutput, nil
}
