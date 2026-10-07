// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAclRootVerityLayoutRequiresUnlockAndUki(t *testing.T) {
	handler := newAclDistroHandler(targetos.TargetOsAzureContainerLinux3)
	rc := &ResolvedConfig{
		PreviewFeatures: []imagecustomizerapi.PreviewFeature{imagecustomizerapi.PreviewFeatureDistroVersion},
		Storage: imagecustomizerapi.Storage{
			Disks: []imagecustomizerapi.Disk{{}},
			Verity: []imagecustomizerapi.Verity{{
				Name:  imagecustomizerapi.VerityRootDeviceName,
				Mount: &imagecustomizerapi.VerityMount{MountPath: "/"},
			}},
		},
	}
	require.ErrorContains(t, handler.ValidateConfig(rc), "unlock-acl")
	assert.False(t, handler.rootVerityLayout)

	rc.PreviewFeatures = append(rc.PreviewFeatures, imagecustomizerapi.PreviewFeatureUnlockAcl)
	require.ErrorContains(t, handler.ValidateConfig(rc), "os.uki.mode: create")

	rc.Uki = &imagecustomizerapi.Uki{Mode: imagecustomizerapi.UkiModeCreate}
	require.NoError(t, handler.ValidateConfig(rc))
	assert.True(t, handler.rootVerityLayout)
	assert.Equal(t, "etc/selinux/config", handler.GetSELinuxConfigFile())
}

func TestUnlockAclOnlyForAcl(t *testing.T) {
	rc := &ResolvedConfig{
		PreviewFeatures: []imagecustomizerapi.PreviewFeature{imagecustomizerapi.PreviewFeatureUnlockAcl},
	}

	for _, targetOs := range []targetos.TargetOs{targetos.TargetOsAzureLinux3, targetos.TargetOsFedora42} {
		handler, err := NewDistroHandler(targetOs)
		require.NoError(t, err)
		require.ErrorContains(t, validateDistroConfig(handler, rc), "only supported for Azure Container Linux")
	}

	acl := newAclDistroHandler(targetos.TargetOsAzureContainerLinux3)
	rc.PreviewFeatures = append(rc.PreviewFeatures, imagecustomizerapi.PreviewFeatureDistroVersion)
	require.NoError(t, validateDistroConfig(acl, rc))
}

func TestMigrateAclEtcToRoot(t *testing.T) {
	rootDir := t.TempDir()
	baseline := filepath.Join(rootDir, aclEtcBaselineDir)
	etc := filepath.Join(rootDir, "etc")
	require.NoError(t, os.MkdirAll(baseline, 0o755))
	require.NoError(t, os.MkdirAll(etc, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(baseline, "fstab"), []byte("old"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(baseline, "passwd"), []byte("factory"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(etc, "fstab"), []byte("new"), 0o644))

	require.NoError(t, migrateAclEtcToRoot(rootDir))
	fstab, err := os.ReadFile(filepath.Join(etc, "fstab"))
	require.NoError(t, err)
	assert.Equal(t, "new", string(fstab))
	passwd, err := os.ReadFile(filepath.Join(etc, "passwd"))
	require.NoError(t, err)
	assert.Equal(t, "factory", string(passwd))
	assert.NoDirExists(t, baseline)
}

// Ensure the ACL package list is populated as an empty (non-nil) list when the
// rpm database can't be read, so the COSI metadata's osPackages field
// serializes as [] rather than null.
func TestAclGetAllPackagesFromChrootReturnsEmptyListWhenNoRpm(t *testing.T) {
	handler := newAclDistroHandler(targetos.TargetOsAzureContainerLinux3)

	// An empty chroot dir has no rpm command and no rpm database.
	chrootDir := t.TempDir()
	chroot := safechroot.NewChroot(chrootDir, true)

	packages, err := handler.GetAllPackagesFromChroot(chroot, nil)
	assert.NoError(t, err)
	assert.NotNil(t, packages)
	assert.Empty(t, packages)
}

// Ensure the ACL package list is populated as an empty (non-nil) list when the
// rpm command exists but the rpm database is missing, so the COSI metadata's
// osPackages field serializes as [] rather than null. This exercises the
// rpm-db-missing branch (the no-rpm test above returns before reaching it).
func TestAclGetAllPackagesFromChrootReturnsEmptyListWhenNoRpmDb(t *testing.T) {
	handler := newAclDistroHandler(targetos.TargetOsAzureContainerLinux3)

	// Provide an rpm command in the chroot PATH but no rpm database.
	chrootDir := t.TempDir()
	rpmBinDir := filepath.Join(chrootDir, "usr", "bin")
	err := os.MkdirAll(rpmBinDir, 0o755)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(rpmBinDir, "rpm"), []byte("#!/bin/sh\n"), 0o755)
	assert.NoError(t, err)

	chroot := safechroot.NewChroot(chrootDir, true)

	packages, err := handler.GetAllPackagesFromChroot(chroot, nil)
	assert.NoError(t, err)
	assert.NotNil(t, packages)
	assert.Empty(t, packages)
}

// Ensure unsupported-distro-version preview feature flag is required when distro version can't be parsed.
func TestCustomizeImageDistroVersionInvalid(t *testing.T) {
	for _, baseImageInfo := range checkSkipForCustomizeDefaultImages(t) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageDistroVersionInvalidHelper(t, "TestCustomizeImageDistroVersionInvalid"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImageDistroVersionInvalidHelper(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTempDir := filepath.Join(tmpDir, testName)
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImage1FilePath := filepath.Join(testTempDir, "image1.qcow2")
	outImage2FilePath := filepath.Join(testTempDir, "image2.qcow2")
	configFile := filepath.Join(testDir, "distro-version-invalid.yaml")

	options := ImageCustomizerOptions{
		BuildDir:             buildDir,
		UseBaseImageRpmRepos: true,
		InputImageFile:       baseImage,
		OutputImageFile:      outImage1FilePath,
		OutputImageFormat:    "qcow2",
		PreviewFeatures:      baseImageInfo.PreviewFeatures,
	}

	// Corrupt the distro version.
	err := CustomizeImageWithConfigFile(t.Context(), configFile, options)
	if !assert.NoError(t, err) {
		return
	}

	options.InputImageFile = outImage1FilePath
	options.OutputImageFile = outImage2FilePath

	// Ensure 'unsupported-distro-version' preview feature flag is enforced.
	configFile = filepath.Join(testDir, "nochange-config.yaml")
	err = CustomizeImageWithConfigFile(t.Context(), configFile, options)
	assert.ErrorIs(t, err, ErrUnsupportedDistroVersion)

	// Enable 'unsupported-distro-version' preview feature flag.
	configFile = filepath.Join(testDir, "distro-version-preview-feature.yaml")
	err = CustomizeImageWithConfigFile(t.Context(), configFile, options)
	if !assert.NoError(t, err) {
		return
	}
}

// Ensure unsupported-distro-version preview feature flag is required when distro version is too new (i.e. very large
// number).
func TestCustomizeImageDistroVersionNew(t *testing.T) {
	for _, baseImageInfo := range checkSkipForCustomizeDefaultImages(t) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageDistroVersionNewHelper(t, "TestCustomizeImageDistroVersionNew"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImageDistroVersionNewHelper(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTempDir := filepath.Join(tmpDir, testName)
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImage1FilePath := filepath.Join(testTempDir, "image1.qcow2")
	outImage2FilePath := filepath.Join(testTempDir, "image2.qcow2")
	configFile := filepath.Join(testDir, "distro-version-new.yaml")

	options := ImageCustomizerOptions{
		BuildDir:             buildDir,
		UseBaseImageRpmRepos: true,
		InputImageFile:       baseImage,
		OutputImageFile:      outImage1FilePath,
		OutputImageFormat:    "qcow2",
		PreviewFeatures:      baseImageInfo.PreviewFeatures,
	}

	// Set the distro version to a very large number.
	err := CustomizeImageWithConfigFile(t.Context(), configFile, options)
	if !assert.NoError(t, err) {
		return
	}

	options.InputImageFile = outImage1FilePath
	options.OutputImageFile = outImage2FilePath

	// Ensure 'unsupported-distro-version' preview feature flag is enforced.
	configFile = filepath.Join(testDir, "nochange-config.yaml")
	err = CustomizeImageWithConfigFile(t.Context(), configFile, options)
	assert.ErrorIs(t, err, ErrUnsupportedDistroVersion)

	// Enable 'unsupported-distro-version' preview feature flag.
	configFile = filepath.Join(testDir, "distro-version-preview-feature.yaml")
	err = CustomizeImageWithConfigFile(t.Context(), configFile, options)
	if !assert.NoError(t, err) {
		return
	}
}

func TestCustomizeImageUnsupportedPackageSnapshotTime(t *testing.T) {
	for _, baseImageInfo := range slices.Concat([]testBaseImageInfo{testBaseImageAzl4CoreEfi}, baseImageUbuntuAll) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageUnsupportedPackageSnapshotTimeHelper(t, baseImageInfo)
		})
	}
}

func testCustomizeImageUnsupportedPackageSnapshotTimeHelper(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageUnsupportedPackageSnapshotTime_"+baseImageInfo.Name)
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")

	options := ImageCustomizerOptions{
		BuildDir:             buildDir,
		InputImageFile:       baseImage,
		OutputImageFile:      "./out/image.vhdx",
		OutputImageFormat:    "vhdx",
		UseBaseImageRpmRepos: true,
		PreviewFeatures:      baseImageInfo.PreviewFeatures,
	}

	config := &imagecustomizerapi.Config{
		PreviewFeatures: []imagecustomizerapi.PreviewFeature{imagecustomizerapi.PreviewFeaturePackageSnapshotTime},
		OS:              &imagecustomizerapi.OS{},
	}

	options.PackageSnapshotTime = "2025-01-01"

	err := CustomizeImage(t.Context(), testTmpDir, config, options)
	assert.ErrorIs(t, err, ErrUnsupportedPackageSnapshotTime)
	assert.ErrorIs(t, err, ErrUnsupportedDistroApi)

	options.PackageSnapshotTime = ""
	config.OS.Packages.SnapshotTime = "2025-01-01"

	err = CustomizeImage(t.Context(), testTmpDir, config, options)
	assert.ErrorIs(t, err, ErrUnsupportedPackageSnapshotTime)
	assert.ErrorIs(t, err, ErrUnsupportedDistroApi)
}

func TestCustomizeImageUnsupportedRpmSources(t *testing.T) {
	for _, baseImageInfo := range baseImageUbuntuAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageUnsupportedRpmSourcesHelper(t, baseImageInfo)
		})
	}
}

func testCustomizeImageUnsupportedRpmSourcesHelper(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageUnsupportedRpmSources_"+baseImageInfo.Name)
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")

	err := os.MkdirAll(testTmpDir, os.ModePerm)
	if !assert.NoError(t, err) {
		return
	}

	repoFile := filepath.Join(testTmpDir, "a.repo")
	err = os.WriteFile(repoFile, []byte{}, os.ModePerm)
	if !assert.NoError(t, err) {
		return
	}

	options := ImageCustomizerOptions{
		BuildDir:             buildDir,
		InputImageFile:       baseImage,
		OutputImageFile:      "./out/image.vhdx",
		OutputImageFormat:    "vhdx",
		UseBaseImageRpmRepos: true,
		PreviewFeatures:      baseImageInfo.PreviewFeatures,
	}

	config := &imagecustomizerapi.Config{
		OS: &imagecustomizerapi.OS{},
	}

	options.RpmsSources = []string{repoFile}

	err = CustomizeImage(t.Context(), testTmpDir, config, options)
	assert.ErrorIs(t, err, ErrUnsupportedRpmSources)
	assert.ErrorIs(t, err, ErrUnsupportedDistroApi)
}
