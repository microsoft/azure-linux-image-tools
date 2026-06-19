// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/stretchr/testify/assert"
)

// packageFile pairs a package with a file it is expected to own. The functional test suite reasons
// about base images both through the package database (via IsPackageInstalled) and through files on
// disk, so the preconditions are asserted against both views.
type packageFile struct {
	packageName string
	filePath    string
}

// baseImagePreconditions captures the assumptions the functional test suite makes about a base
// image. It is the in-tree port of the checks previously performed out-of-tree by validate-image.sh.
type baseImagePreconditions struct {
	// packagesAbsent are installed and then asserted by the packages-add / packages-update tests. If
	// a base image already ships them, those tests prove nothing.
	packagesAbsent []packageFile

	// packagesPresent are assumed to already be installed by other tests.
	packagesPresent []packageFile
}

// preconditionsForBaseImage returns the preconditions a base image must satisfy. The packages-add /
// packages-update tests install tree on every distro, plus a second package that differs by distro:
// Azure Linux 4.0 uses jq (its base image already ships unzip) while the others use unzip.
func preconditionsForBaseImage(baseImageInfo testBaseImageInfo) baseImagePreconditions {
	secondAbsentPackage := packageFile{packageName: "unzip", filePath: "/usr/bin/unzip"}
	if baseImageInfo.Distro == baseImageDistroAzureLinux && baseImageInfo.Version == baseImageVersionAzl4 {
		secondAbsentPackage = packageFile{packageName: "jq", filePath: "/usr/bin/jq"}
	}

	return baseImagePreconditions{
		packagesAbsent: []packageFile{
			{packageName: "tree", filePath: "/usr/bin/tree"},
			secondAbsentPackage,
		},
		packagesPresent: []packageFile{
			{packageName: "chrony", filePath: "/usr/sbin/chronyd"},
			{packageName: "curl", filePath: "/usr/bin/curl"},
		},
	}
}

// TestBaseImagePreconditions validates the assumptions the functional tests make about each base
// image: that packages the packages-add / packages-update tests install are absent, that packages
// other tests rely on are present, and that the base image does not already set both console=tty0
// and console=ttyS0 (which would make the extraCommandLine console assertions vacuous).
//
// This is the in-tree replacement for the validate-image.sh MCP helper. Converting the base image to
// a throwaway raw copy, connecting via the loopback chroot, and querying packages through the
// package-manager abstraction replaces the script's bespoke qemu-img + losetup + mount + chroot and
// its per-distro package-manager and root-partition branching.
func TestBaseImagePreconditions(t *testing.T) {
	var baseImages []testBaseImageInfo
	baseImages = append(baseImages, baseImageAzureLinuxAll...)
	baseImages = append(baseImages, baseImageUbuntuAll...)

	for _, baseImageInfo := range baseImages {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testBaseImagePreconditions(t, baseImageInfo)
		})
	}
}

func testBaseImagePreconditions(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, "TestBaseImagePreconditions"+baseImageInfo.Name)
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	rawImageFilePath := filepath.Join(testTmpDir, "image.raw")

	err := os.MkdirAll(testTmpDir, os.ModePerm)
	if !assert.NoError(t, err) {
		return
	}

	// Convert the base image to a throwaway raw copy so the shared base image is never mounted
	// writable, then connect to that copy. Default mounts are required because the package-manager
	// queries below exec tdnf/rpm/dpkg-query inside the chroot.
	err = ConvertImageFile(baseImage, rawImageFilePath, "raw")
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err := testutils.ConnectToImage(buildDir, rawImageFilePath, true /*includeDefaultMounts*/, baseImageInfo.MountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	distroHandler, err := NewDistroHandlerFromChroot(imageConnection.Chroot())
	if !assert.NoError(t, err) {
		return
	}

	rootDir := imageConnection.Chroot().RootDir()
	preconditions := preconditionsForBaseImage(baseImageInfo)

	// Packages the packages-add / packages-update tests install must not already be present.
	for _, pkg := range preconditions.packagesAbsent {
		assert.Falsef(t, distroHandler.IsPackageInstalled(imageConnection.Chroot(), pkg.packageName),
			"package %q must be absent so the packages-add / packages-update tests are meaningful", pkg.packageName)
		assertImageFileAbsent(t, rootDir, pkg.filePath)
	}

	// Packages other tests assume are pre-installed must be present.
	for _, pkg := range preconditions.packagesPresent {
		assert.Truef(t, distroHandler.IsPackageInstalled(imageConnection.Chroot(), pkg.packageName),
			"package %q must be pre-installed", pkg.packageName)
		assertImageFilePresent(t, rootDir, pkg.filePath)
	}

	// The extraCommandLine console tests add console=tty0 and console=ttyS0 and assert they appear
	// together. If the base image already sets both, those assertions are vacuous.
	assertConsoleArgsNotBothPreset(t, rootDir)
}

func assertImageFileAbsent(t *testing.T, rootDir string, imagePath string) {
	exists, err := file.PathExists(filepath.Join(rootDir, imagePath))
	if assert.NoErrorf(t, err, "failed to check for %q", imagePath) {
		assert.Falsef(t, exists, "%q must be absent", imagePath)
	}
}

func assertImageFilePresent(t *testing.T, rootDir string, imagePath string) {
	exists, err := file.PathExists(filepath.Join(rootDir, imagePath))
	if assert.NoErrorf(t, err, "failed to check for %q", imagePath) {
		assert.Truef(t, exists, "%q must be present", imagePath)
	}
}

// assertConsoleArgsNotBothPreset fails if the base image already sets both console=tty0 and
// console=ttyS0 in either /etc/default/grub or /etc/kernel/cmdline. Having at least one absent keeps
// the extraCommandLine console-argument assertions meaningful.
func assertConsoleArgsNotBothPreset(t *testing.T, rootDir string) {
	consoleArgFiles := []string{"etc/default/grub", "etc/kernel/cmdline"}

	tty0Present := false
	ttyS0Present := false
	for _, relativePath := range consoleArgFiles {
		fullPath := filepath.Join(rootDir, relativePath)

		exists, err := file.PathExists(fullPath)
		if !assert.NoErrorf(t, err, "failed to check for %q", relativePath) {
			return
		}
		if !exists {
			continue
		}

		contents, err := os.ReadFile(fullPath)
		if !assert.NoErrorf(t, err, "failed to read %q", relativePath) {
			return
		}

		if strings.Contains(string(contents), "console=tty0") {
			tty0Present = true
		}
		if strings.Contains(string(contents), "console=ttyS0") {
			ttyS0Present = true
		}
	}

	assert.Falsef(t, tty0Present && ttyS0Present,
		"base image already sets both console=tty0 and console=ttyS0, making the extraCommandLine console tests vacuous")
}
