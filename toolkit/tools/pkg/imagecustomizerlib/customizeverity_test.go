// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safemount"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

func TestCustomizeImageVerity(t *testing.T) {
	for _, version := range supportedAzureLinuxVersions {
		t.Run(string(version), func(t *testing.T) {
			testCustomizeImageVerityHelper(t, "TestCustomizeImageVerity"+string(version), baseImageTypeCoreEfi,
				version)
		})
	}
}

func testCustomizeImageVerityHelper(t *testing.T, testName string, imageType baseImageType,
	imageVersion baseImageVersion,
) {
	baseImage := checkSkipForCustomizeImage(t, imageType, imageVersion)

	testTempDir := filepath.Join(tmpDir, testName)
	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	configFile := filepath.Join(testDir, "verity-config.yaml")

	// Customize image.
	err := CustomizeImageWithConfigFile(buildDir, configFile, baseImage, nil, outImageFilePath, "raw", "",
		"" /*outputPXEArtifactsDir*/, true /*useBaseImageRpmRepos*/, false /*enableShrinkFilesystems*/)
	if !assert.NoError(t, err) {
		return
	}

	verityRootVerity(t, imageType, imageVersion, buildDir, outImageFilePath)

	// Recustomize the image.
	err = CustomizeImageWithConfigFile(buildDir, configFile, outImageFilePath, nil, outImageFilePath, "raw", "",
		"" /*outputPXEArtifactsDir*/, true /*useBaseImageRpmRepos*/, false /*enableShrinkFilesystems*/)
	if !assert.NoError(t, err) {
		return
	}

	verityRootVerity(t, imageType, imageVersion, buildDir, outImageFilePath)
}

func verityRootVerity(t *testing.T, imageType baseImageType, imageVersion baseImageVersion, buildDir string,
	outImageFilePath string,
) {
	// Connect to customized image.
	mountPoints := []mountPoint{
		{
			PartitionNum:   3,
			Path:           "/",
			FileSystemType: "ext4",
			Flags:          unix.MS_RDONLY,
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
		{
			PartitionNum:   5,
			Path:           "/var",
			FileSystemType: "ext4",
		},
	}

	imageConnection, err := connectToImage(buildDir, outImageFilePath, false /*includeDefaultMounts*/, mountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	partitions, err := getDiskPartitionsMap(imageConnection.Loopback().DevicePath())
	assert.NoError(t, err, "get disk partitions")

	// Verify that verity is configured correctly.
	// This helps verify that verity-enabled images can be recustomized.
	bootPath := filepath.Join(imageConnection.chroot.RootDir(), "/boot")
	rootDevice := partitionDevPath(imageConnection, 3)
	hashDevice := partitionDevPath(imageConnection, 4)
	verifyVerityGrub(t, bootPath, rootDevice, hashDevice, "PARTUUID="+partitions[3].PartUuid,
		"PARTUUID="+partitions[4].PartUuid, "root", "rd.info", imageVersion)

	err = imageConnection.CleanClose()
	if !assert.NoError(t, err) {
		return
	}
}

func TestCustomizeImageVerityShrinkExtract(t *testing.T) {
	for _, version := range supportedAzureLinuxVersions {
		t.Run(string(version), func(t *testing.T) {
			testCustomizeImageVerityShrinkExtractHelper(t, "TestCustomizeImageVerityShrinkExtract"+string(version),
				baseImageTypeCoreEfi, version)
		})
	}
}

func testCustomizeImageVerityShrinkExtractHelper(t *testing.T, testName string, imageType baseImageType,
	imageVersion baseImageVersion,
) {
	baseImage := checkSkipForCustomizeImage(t, imageType, imageVersion)

	testTempDir := filepath.Join(tmpDir, testName)
	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	configFile := filepath.Join(testDir, "verity-partition-labels.yaml")

	var config imagecustomizerapi.Config
	err := imagecustomizerapi.UnmarshalAndValidateYamlFile(configFile, &config)
	if !assert.NoError(t, err) {
		return
	}

	espPartitionNum := 1
	bootPartitionNum := 2
	rootPartitionNum := 3
	hashPartitionNum := 4
	varPartitionNum := 5

	// Customize image, shrink partitions, and split the partitions into individual files.
	err = CustomizeImage(buildDir, testDir, &config, baseImage, nil, outImageFilePath, "", "raw",
		"" /*outputPXEArtifactsDir*/, true /*useBaseImageRpmRepos*/, true /*enableShrinkFilesystems*/)
	if !assert.NoError(t, err) {
		return
	}

	espPartitionPath := filepath.Join(testTempDir, fmt.Sprintf("image_%d.raw", espPartitionNum))
	bootPartitionPath := filepath.Join(testTempDir, fmt.Sprintf("image_%d.raw", bootPartitionNum))
	rootPartitionPath := filepath.Join(testTempDir, fmt.Sprintf("image_%d.raw", rootPartitionNum))
	hashPartitionPath := filepath.Join(testTempDir, fmt.Sprintf("image_%d.raw", hashPartitionNum))
	varPartitionPath := filepath.Join(testTempDir, fmt.Sprintf("image_%d.raw", varPartitionNum))

	espStat, err := os.Stat(espPartitionPath)
	assert.NoError(t, err)

	bootStat, err := os.Stat(bootPartitionPath)
	assert.NoError(t, err)

	rootStat, err := os.Stat(rootPartitionPath)
	assert.NoError(t, err)

	hashStat, err := os.Stat(hashPartitionPath)
	assert.NoError(t, err)

	varStat, err := os.Stat(varPartitionPath)
	assert.NoError(t, err)

	// Check partition sizes.
	assert.Equal(t, int64(8*diskutils.MiB), espStat.Size())
	assert.Equal(t, int64(100*diskutils.MiB), hashStat.Size())

	// These partitions are shrunk.
	// So, their size will vary slightly.
	assert.Greater(t, int64(100*diskutils.MiB), bootStat.Size())
	assert.Greater(t, int64(650*diskutils.MiB), rootStat.Size())
	assert.Greater(t, int64(150*diskutils.MiB), varStat.Size())

	// Attach partition files.
	bootDevice, err := safeloopback.NewLoopback(bootPartitionPath)
	if !assert.NoError(t, err) {
		return
	}
	defer bootDevice.Close()

	rootDevice, err := safeloopback.NewLoopback(rootPartitionPath)
	if !assert.NoError(t, err) {
		return
	}
	defer rootDevice.Close()

	hashDevice, err := safeloopback.NewLoopback(hashPartitionPath)
	if !assert.NoError(t, err) {
		return
	}
	defer hashDevice.Close()

	bootMountPath := filepath.Join(buildDir, "bootpartition")
	bootMount, err := safemount.NewMount(bootDevice.DevicePath(), bootMountPath, "ext4", 0, "", true)
	if !assert.NoError(t, err) {
		return
	}
	defer bootMount.Close()

	// Verify that verity is configured correctly.
	verifyVerityGrub(t, bootMountPath, rootDevice.DevicePath(), hashDevice.DevicePath(), "PARTLABEL=root",
		"PARTLABEL=roothash", "root", "rd.info", imageVersion)
}

func verifyVerityGrub(t *testing.T, bootPath string, dataDevice string, hashDevice string, dataId string, hashId string,
	verityType string, extraCommandLine string, imageVersion baseImageVersion,
) {
	// Extract kernel command line args.
	grubCfgPath := filepath.Join(bootPath, "/grub2/grub.cfg")
	grubCfgContents, err := file.Read(grubCfgPath)
	if !assert.NoError(t, err) {
		return
	}

	argsRegexp := regexp.MustCompile(`(?m)^[\t ]*linux[\t ]*\S+[\t ]*(.*)$`)
	matches := argsRegexp.FindAllStringSubmatch(grubCfgContents, -1)

	kernelArgsList := []string(nil)
	for _, match := range matches {
		kernelArgsList = append(kernelArgsList, match[1])
	}

	// Verify verity.
	verifyVerityHelper(t, kernelArgsList, dataDevice, hashDevice, dataId, hashId, verityType)

	// Verity extra command line args.
	recoveryCount := 0
	if imageVersion == baseImageVersionAzl3 {
		// Count the number of recovery menu items there are.
		// These menu items won't contain the extra command line args.
		recoveryCount = strings.Count(grubCfgContents, "(recovery mode)")
	}

	cmdlineRegexp := regexp.MustCompile(fmt.Sprintf(` %s( |$)`, regexp.QuoteMeta(extraCommandLine)))

	// Count the number of linux lines contain the extra command line args.
	extracCommandLineMatchCount := 0
	for _, kernelArgs := range kernelArgsList {
		if cmdlineRegexp.MatchString(kernelArgs) {
			extracCommandLineMatchCount += 1
		}
	}

	assert.Equal(t, len(kernelArgsList)-recoveryCount, extracCommandLineMatchCount)
}

func verifyVerityUki(t *testing.T, espPath string, dataDevice string,
	hashDevice string, dataId string, hashId string, verityType string, buildDir string, extraCommandLine string,
) {
	// Extract kernel command line args.
	kernelArgsList, err := extractKernelCmdlineFromUkiEfis(espPath, buildDir)
	if !assert.NoError(t, err) {
		return
	}

	// Verify verity
	verifyVerityHelper(t, kernelArgsList, dataDevice, hashDevice, dataId, hashId, verityType)

	// Verify extra command line
	for _, kernelArgs := range kernelArgsList {
		assert.Regexp(t, fmt.Sprintf(` %s( |$)`, regexp.QuoteMeta(extraCommandLine)), kernelArgs)
	}
}

func verifyVerityHelper(t *testing.T, kernelArgsList []string, dataDevice string,
	hashDevice string, dataId string, hashId string, verityType string,
) {
	assert.GreaterOrEqual(t, len(kernelArgsList), 1)

	hash := ""
	for _, kernelArgs := range kernelArgsList {
		var hashRegexp *regexp.Regexp
		switch verityType {
		case "root":
			assert.Regexp(t, ` rd.systemd.verity=1 `, kernelArgs)
			assert.Regexp(t, fmt.Sprintf(` systemd.verity_root_data=%s `, dataId), kernelArgs)
			assert.Regexp(t, fmt.Sprintf(` systemd.verity_root_hash=%s `, hashId), kernelArgs)
			assert.Regexp(t, ` systemd.verity_root_options=panic-on-corruption( |$)`, kernelArgs)

			hashRegexp = regexp.MustCompile(` roothash=([a-fA-F0-9]*) `)

		case "usr":
			assert.Regexp(t, ` rd.systemd.verity=1 `, kernelArgs)
			assert.Regexp(t, fmt.Sprintf(` systemd.verity_usr_data=%s `, dataId), kernelArgs)
			assert.Regexp(t, fmt.Sprintf(` systemd.verity_usr_hash=%s `, hashId), kernelArgs)
			assert.Regexp(t, ` systemd.verity_usr_options=panic-on-corruption( |$)`, kernelArgs)

			hashRegexp = regexp.MustCompile(` usrhash=([a-fA-F0-9]*) `)

		default:
			t.Errorf("Invalid verity type: (%s)", verityType)
		}

		hashMatches := hashRegexp.FindStringSubmatch(kernelArgs)
		if !assert.Equal(t, 2, len(hashMatches)) {
			continue
		}

		kernelArgsHash := hashMatches[1]
		if hash == "" {
			hash = kernelArgsHash
		} else {
			// Ensure all the hashes are the same for all kernel versions.
			assert.Equal(t, hash, kernelArgsHash)
		}
	}

	if assert.NotEqual(t, "", hash) {
		// Verify verity hashes.
		err := shell.ExecuteLive(false, "veritysetup", "verify", dataDevice, hashDevice, hash)
		assert.NoError(t, err)
	}
}

func TestCustomizeImageVerityUsr(t *testing.T) {
	for _, version := range supportedAzureLinuxVersions {
		t.Run(string(version), func(t *testing.T) {
			testCustomizeImageVerityUsrHelper(t, "TestCustomizeImageVerityUsr"+string(version), baseImageTypeCoreEfi,
				version)
		})
	}
}

func testCustomizeImageVerityUsrHelper(t *testing.T, testName string, imageType baseImageType,
	imageVersion baseImageVersion,
) {
	baseImage := checkSkipForCustomizeImage(t, imageType, imageVersion)

	testTempDir := filepath.Join(tmpDir, testName)
	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	configFile := filepath.Join(testDir, "verity-usr-config.yaml")

	// Customize image.
	err := CustomizeImageWithConfigFile(buildDir, configFile, baseImage, nil, outImageFilePath, "raw", "",
		"" /*outputPXEArtifactsDir*/, true /*useBaseImageRpmRepos*/, false /*enableShrinkFilesystems*/)
	if !assert.NoError(t, err) {
		return
	}

	verityUsrVerity(t, imageType, imageVersion, buildDir, outImageFilePath)

	// Recustomize image.
	// This helps verify that verity-enabled images can be recustomized.
	err = CustomizeImageWithConfigFile(buildDir, configFile, outImageFilePath, nil, outImageFilePath, "raw", "",
		"" /*outputPXEArtifactsDir*/, true /*useBaseImageRpmRepos*/, false /*enableShrinkFilesystems*/)
	if !assert.NoError(t, err) {
		return
	}

	verityUsrVerity(t, imageType, imageVersion, buildDir, outImageFilePath)
}

func verityUsrVerity(t *testing.T, imageType baseImageType, imageVersion baseImageVersion, buildDir string,
	outImageFilePath string,
) {
	// Connect to usr verity image.
	mountPoints := []mountPoint{
		{
			PartitionNum:   5,
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
		{
			PartitionNum:   3,
			Path:           "/usr",
			FileSystemType: "ext4",
			Flags:          unix.MS_RDONLY,
		},
	}

	imageConnection, err := connectToImage(buildDir, outImageFilePath, false /*includeDefaultMounts*/, mountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	partitions, err := getDiskPartitionsMap(imageConnection.Loopback().DevicePath())
	assert.NoError(t, err, "get disk partitions")

	// Verify that usr verity is configured correctly.
	bootPath := filepath.Join(imageConnection.chroot.RootDir(), "/boot")
	usrDevice := partitionDevPath(imageConnection, 3)
	hashDevice := partitionDevPath(imageConnection, 4)
	verifyVerityGrub(t, bootPath, usrDevice, hashDevice, "PARTUUID="+partitions[3].PartUuid,
		"PARTUUID="+partitions[4].PartUuid, "usr", "rd.info", imageVersion)
}
