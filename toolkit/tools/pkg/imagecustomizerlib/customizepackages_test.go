// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/sliceutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
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

	// Install unzip package.
	config := imagecustomizerapi.Config{
		OS: &imagecustomizerapi.OS{
			Packages: imagecustomizerapi.Packages{
				Install: []string{"unzip"},
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

	// Ensure unzip was installed.
	ensureFilesExist(t, imageConnection,
		"/usr/bin/unzip",
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

	// Create a copy of the RPMs directory, but without the unzip package.
	// This ensures that the package repo metadata is refreshed between runs.
	err = os.RemoveAll(downloadedRpmsTmpDir)
	if !assert.NoError(t, err) {
		return
	}

	err = copyRpms(downloadedRpmsDir, downloadedRpmsTmpDir, []string{"unzip-"})
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

	// Ensure tree was installed.
	ensureFilesExist(t, imageConnection,
		"/usr/bin/unzip",
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
		if sourceFile.IsDir() {
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

	downloadedRpmsRepoFile := testutils.GetDownloadedRpmsRepoFile(t, testutilsDir, baseImageInfo.Distro,
		baseImageInfo.Version, withGpgKey, false)
	rpmSources := []string{downloadedRpmsRepoFile}

	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	configFile := filepath.Join(testDir, "packages-add-config.yaml")

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

	// Ensure packages were installed.
	ensureFilesExist(t, imageConnection,
		"/usr/bin/unzip",
		"/usr/bin/tree",
	)
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
	configFile := filepath.Join(testDir, "packages-update-config.yaml")

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

	ensureFilesExist(t, imageConnection,
		"/usr/bin/unzip",
	)

	if baseImageInfo.Distro == baseImageDistroUbuntu {
		ensureAptCacheCleanup(t, imageConnection)
		ensureAptServicePreventionRestored(t, imageConnection)
	} else {
		ensureTdnfCacheCleanup(t, imageConnection, "/var/cache/tdnf", baseImageInfo)
	}
}

func TestCustomizeImagePackagesUpdateExistingOnline(t *testing.T) {
	for _, baseImageInfo := range checkSkipForCustomizeDefaultImages(t) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImagePackagesUpdateExisting(t, baseImageInfo)
		})
	}
}

func testCustomizeImagePackagesUpdateExisting(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

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

	if baseImageInfo.Distro == baseImageDistroUbuntu {
		ensureAptCacheCleanup(t, imageConnection)
		ensureAptServicePreventionRestored(t, imageConnection)
	} else {
		ensureTdnfCacheCleanup(t, imageConnection, "/var/cache/tdnf", baseImageInfo)
	}
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
	configFile := filepath.Join(testDir, "packages-remove-config.yaml")

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

	// Verify both inline (python3) and list-referenced (gzip) packages were removed.
	ensureFilesNotExist(t, imageConnection,
		"/usr/bin/python3",
		"/usr/bin/gzip",
	)

	if baseImageInfo.Distro == baseImageDistroUbuntu {
		ensureAptCacheCleanup(t, imageConnection)
		ensureAptServicePreventionRestored(t, imageConnection)
	} else {
		ensureTdnfCacheCleanup(t, imageConnection, "/var/cache/tdnf", baseImageInfo)
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

	// Verify dpkg log is truncated to zero bytes.
	dpkgLogPath := filepath.Join(rootDir, "var/log/dpkg.log")
	info, err := os.Stat(dpkgLogPath)
	assert.NoError(t, err, "failed to stat dpkg log file")
	assert.Equal(t, int64(0), info.Size(), "expected dpkg.log to be truncated (0 bytes), but got %d bytes", info.Size())
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
	baseImage, _ := checkSkipForCustomizeDefaultAzureLinuxImage(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImagePackagesDiskSpace")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")

	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	configFile := filepath.Join(testDir, "install-package-disk-space.yaml")

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.ErrorContains(t, err, "failed to customize OS")
	assert.ErrorContains(t, err, "failed to install packages ([gcc])")
}

func TestCustomizeImagePackagesUrlSource(t *testing.T) {
	baseImageInfo := testBaseImageAzl3CoreEfi
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImagePackagesUrlSource")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")

	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	configFile := filepath.Join(testDir, "packages-add-oras.yaml")

	repoFile := filepath.Join(testDir, "repos/cloud-native-azl3.repo")

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
	baseImageInfo := testBaseImageAzl3CoreEfi
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImagePackagesBadRepo")
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

func ensureTdnfCacheCleanup(t *testing.T, imageConnection *imageconnection.ImageConnection, dirPath string,
	baseImageInfo testBaseImageInfo,
) {
	// Array to capture all the files of the provided root directory
	var existingFiles []string

	// Start the directory walk from the initial dirPath and collect all existing files
	fullPath := filepath.Join(imageConnection.Chroot().RootDir(), dirPath)
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
	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImagePackagesSnapshotTime")
	defer os.RemoveAll(testTmpDir)

	baseImageInfo := testBaseImageAzl3CoreEfi
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)
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
	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImagePackagesCliSnapshotTimeOverridesConfigFile")
	defer os.RemoveAll(testTmpDir)

	baseImageInfo := testBaseImageAzl3CoreEfi
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)
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
	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImagePackagesSnapshotTimeWithoutPreviewFlagFails")
	defer os.RemoveAll(testTmpDir)

	baseImageInfo := testBaseImageAzl3CoreEfi
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

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
	configFile := filepath.Join(testDir, "packages-add-config.yaml")

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

	// Verify both inline (unzip) and list-referenced (tree) packages were installed.
	ensureFilesExist(t, imageConnection,
		"/usr/bin/unzip",
		"/usr/bin/tree",
	)

	if baseImageInfo.Distro == baseImageDistroUbuntu {
		ensureAptCacheCleanup(t, imageConnection)
		ensureAptServicePreventionRestored(t, imageConnection)
	} else {
		ensureTdnfCacheCleanup(t, imageConnection, "/var/cache/tdnf", baseImageInfo)
	}
}

func getPkgVersionFromChroot(imageConnection *imageconnection.ImageConnection, pkgName string) (string, error) {
	var versionOutput string
	err := imageConnection.Chroot().UnsafeRun(func() error {
		out, _, err := shell.Execute("rpm", "-q", pkgName)
		if err != nil {
			return fmt.Errorf("failed to query rpm:\n%w", err)
		}
		versionOutput = strings.TrimSpace(out)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to get version of %s in chroot:\n%w", pkgName, err)
	}

	return versionOutput, nil
}
