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

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/sliceutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/testutils"
	"github.com/stretchr/testify/assert"
)

func TestCustomizeImagePackagesAddOfflineDir(t *testing.T) {
	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImagePackagesAddOfflineDir")

	baseImage, _ := checkSkipForCustomizeDefaultImage(t)
	downloadedRpmsDir := testutils.GetDownloadedRpmsDir(t, testutilsDir, "2.0", false)
	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	downloadedRpmsTmpDir := filepath.Join(testTmpDir, "rpms")

	// Create a copy of the RPMs directory, but without the golang package.
	err := copyRpms(downloadedRpmsDir, downloadedRpmsTmpDir, []string{"golang-"})
	if !assert.NoError(t, err) {
		return
	}

	// Install jq package.
	config := imagecustomizerapi.Config{
		OS: &imagecustomizerapi.OS{
			Packages: imagecustomizerapi.Packages{
				Install: []string{"jq"},
			},
		},
	}

	err = CustomizeImage(t.Context(), buildDir, testDir, &config, baseImage, []string{downloadedRpmsTmpDir}, outImageFilePath,
		"raw", false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err := connectToCoreEfiImage(buildDir, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Ensure jq was installed.
	ensureFilesExist(t, imageConnection,
		"/usr/bin/jq",
	)

	verifyImageHistoryFile(t, 1, config, imageConnection.Chroot().RootDir())

	err = imageConnection.CleanClose()
	if !assert.NoError(t, err) {
		return
	}

	// Create a copy of the RPMs directory, but without the jq package.
	// This ensures that the package repo metadata is refreshed between runs.
	err = os.RemoveAll(downloadedRpmsTmpDir)
	if !assert.NoError(t, err) {
		return
	}

	err = copyRpms(downloadedRpmsDir, downloadedRpmsTmpDir, []string{"jq-"})
	if !assert.NoError(t, err) {
		return
	}

	// Install jq package.
	config = imagecustomizerapi.Config{
		OS: &imagecustomizerapi.OS{
			Packages: imagecustomizerapi.Packages{
				InstallLists: []string{"lists/golang.yaml"},
			},
		},
	}

	err = CustomizeImage(t.Context(), buildDir, testDir, &config, outImageFilePath, []string{downloadedRpmsTmpDir}, outImageFilePath,
		"raw", false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err = connectToCoreEfiImage(buildDir, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Ensure go was installed.
	ensureFilesExist(t, imageConnection,
		"/usr/bin/jq",
		"/usr/bin/go",
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

	baseImage, _ := checkSkipForCustomizeDefaultImage(t)

	downloadedRpmsRepoFile := testutils.GetDownloadedRpmsRepoFile(t, testutilsDir, "2.0", withGpgKey, false)
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

	imageConnection, err := connectToCoreEfiImage(buildDir, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Ensure packages were installed.
	ensureFilesExist(t, imageConnection,
		"/usr/bin/jq",
		"/usr/bin/go",
	)
}

func TestCustomizeImagePackagesUpdate(t *testing.T) {
	baseImage, baseImageInfo := checkSkipForCustomizeDefaultImage(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImagePackagesUpdate")
	buildDir := filepath.Join(testTmpDir, "build")

	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	configFile := filepath.Join(testDir, "packages-update-config.yaml")

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err := connectToCoreEfiImage(buildDir, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Ensure tdnf cache was cleaned.
	ensureTdnfCacheCleanup(t, imageConnection, "/var/cache/tdnf", baseImageInfo)

	// Ensure packages were installed.
	ensureFilesExist(t, imageConnection,
		"/usr/bin/jq",
		"/usr/bin/go",
	)

	// Ensure packages were removed.
	ensureFilesNotExist(t, imageConnection,
		"/usr/bin/which")
}

func TestCustomizeImagePackagesDiskSpace(t *testing.T) {
	baseImage, _ := checkSkipForCustomizeDefaultImage(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImagePackagesDiskSpace")
	buildDir := filepath.Join(testTmpDir, "build")

	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	configFile := filepath.Join(testDir, "install-package-disk-space.yaml")

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.ErrorContains(t, err, "failed to customize raw image")
	assert.ErrorContains(t, err, "failed to install packages (packages=[gcc])")
}

func TestCustomizeImagePackagesUrlSource(t *testing.T) {
	baseImageInfo := testBaseImageAzl3CoreEfi
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImagePackagesUrlSource")
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

	imageConnection, err := connectToCoreEfiImage(buildDir, outImageFilePath)
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
	buildDir := filepath.Join(testTmpDir, "build")

	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	configFile := filepath.Join(testDir, "packages-add-oras.yaml")

	repoFile := filepath.Join(testDir, "repos/bad-repo.repo")

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, []string{repoFile}, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.ErrorContains(t, err, "failed to refresh tdnf repo metadata")
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

	baseImageInfo := testBaseImageAzl3CoreEfi
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)
	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	// Set the snapshot time to a date before jq-1.7.1-2 (2025-03-18) was published, so jq-1.7.1-1 is expected
	snapshotTime := "2025-01-01"

	config := imagecustomizerapi.Config{
		PreviewFeatures: []imagecustomizerapi.PreviewFeature{
			imagecustomizerapi.PreviewFeaturePackageSnapshotTime,
		},
		OS: &imagecustomizerapi.OS{
			Packages: imagecustomizerapi.Packages{
				Install:      []string{"jq"},
				SnapshotTime: imagecustomizerapi.PackageSnapshotTime(snapshotTime),
			},
		},
	}

	err := CustomizeImage(t.Context(), buildDir, testDir, &config, baseImage, nil, outImageFilePath,
		"raw", true, "")
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, true /*includeDefaultMounts*/, coreEfiMountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	ensureFilesExist(t, imageConnection,
		"/usr/bin/jq",
	)

	jqVersionOutput, err := getPkgVersionFromChroot(imageConnection, "jq")
	assert.NoError(t, err, "failed to retrieve jq version from chroot")

	expectedVersion := "jq-1.7.1-1"
	assert.Containsf(t, jqVersionOutput, expectedVersion,
		"snapshotTime %s should install jq version %s, but got: %s", snapshotTime, expectedVersion, jqVersionOutput)

	ensureFilesNotExist(t, imageConnection, customTdnfConfRelPath)

	verifyImageHistoryFile(t, 1, config, imageConnection.Chroot().RootDir())
}

func TestCustomizeImagePackagesCliSnapshotTimeOverridesConfigFile(t *testing.T) {
	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImagePackagesSnapshotTime")

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
				Install:      []string{"jq"},
				SnapshotTime: imagecustomizerapi.PackageSnapshotTime(snapshotTimeConfig),
			},
		},
	}

	// Set the snapshot time in CLI to a date before jq-1.7.1-2 (2025-03-18) was published
	err := CustomizeImage(t.Context(), buildDir, testDir, &config, baseImage, nil, outImageFilePath,
		"raw", true, snapshotTimeCLI)
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, true /*includeDefaultMounts*/, coreEfiMountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	ensureFilesExist(t, imageConnection,
		"/usr/bin/jq",
	)

	jqVersionOutput, err := getPkgVersionFromChroot(imageConnection, "jq")
	assert.NoError(t, err, "failed to retrieve jq version from chroot")

	expectedVersion := "jq-1.7.1-1"
	assert.Containsf(t, jqVersionOutput, expectedVersion,
		"snapshotTime %s should install jq version %s, but got: %s", snapshotTimeCLI, expectedVersion, jqVersionOutput)

	ensureFilesNotExist(t, imageConnection, customTdnfConfRelPath)

	verifyImageHistoryFile(t, 1, config, imageConnection.Chroot().RootDir())
}

func TestCustomizeImagePackagesSnapshotTimeWithoutPreviewFlagFails(t *testing.T) {
	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImagePackagesSnapshotTimeWithoutPreviewFlagFails")

	baseImageInfo := testBaseImageAzl3CoreEfi
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	config := imagecustomizerapi.Config{
		OS: &imagecustomizerapi.OS{
			Packages: imagecustomizerapi.Packages{
				Install:      []string{"jq"},
				SnapshotTime: "2025-05-22",
			},
		},
	}

	err := CustomizeImage(t.Context(), buildDir, testDir, &config, baseImage, nil, outImageFilePath,
		"raw", true, "")
	assert.ErrorContains(t, err, "snapshotTime")
	assert.ErrorContains(t, err, "preview feature")
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
