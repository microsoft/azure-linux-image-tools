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

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

func TestCustomizeImageVerity(t *testing.T) {
	for _, baseImageInfo := range baseImageAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageVerityHelper(t, "TestCustomizeImageVerity"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImageVerityHelper(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTempDir := filepath.Join(tmpDir, testName)
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	configFile := filepath.Join(testDir, "verity-config.yaml")

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	verifyRootVerity(t, baseImageInfo, buildDir, outImageFilePath)

	// Recustomize the image.
	err = CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, outImageFilePath, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	verifyRootVerity(t, baseImageInfo, buildDir, outImageFilePath)
}

func verifyRootVerity(t *testing.T, baseImageInfo testBaseImageInfo, buildDir string,
	outImageFilePath string,
) {
	// Connect to customized image.
	mountPoints := []testutils.MountPoint{
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

	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, false /*includeDefaultMounts*/, mountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	partitions, err := getDiskPartitionsMap(imageConnection.Loopback().DevicePath())
	assert.NoError(t, err, "get disk partitions")

	// Verify that verity is configured correctly.
	// This helps verify that verity-enabled images can be recustomized.
	bootPath := filepath.Join(imageConnection.Chroot().RootDir(), "/boot")
	rootDevice := testutils.PartitionDevPath(imageConnection, 3)
	hashDevice := testutils.PartitionDevPath(imageConnection, 4)
	verifyVerityGrub(t, bootPath, rootDevice, hashDevice, "PARTUUID="+partitions[3].PartUuid,
		"PARTUUID="+partitions[4].PartUuid, "root", "rd.info", baseImageInfo, "panic-on-corruption")

	err = imageConnection.CleanClose()
	if !assert.NoError(t, err) {
		return
	}
}

func TestCustomizeImageVerityCosiShrinkExtract(t *testing.T) {
	for _, baseImageInfo := range baseImageAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageVerityCosiExtractHelper(t, "TestCustomizeImageVerityShrinkExtract"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImageVerityCosiExtractHelper(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTempDir := filepath.Join(tmpDir, testName)
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.cosi")
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
	err = CustomizeImage(t.Context(), buildDir, testDir, &config, baseImage, nil, outImageFilePath, "cosi",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)

	if !assert.NoError(t, err) {
		return
	}

	// Attach partition files.
	partitionsPaths, err := extractPartitionsFromCosi(outImageFilePath, testTempDir)
	if !assert.NoError(t, err) || !assert.Len(t, partitionsPaths, 5) {
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

	// These partitions are shrunk. Their final size will vary based on base image version, package versions, filesystem
	// implementation details, and randomness. So, just enforce that the final size is below an arbitary value. Values
	// were picked by observing values seen during test and adding a good buffer.
	assert.Greater(t, int64(150*diskutils.MiB), bootStat.Size())
	assert.Greater(t, int64(675*diskutils.MiB), rootStat.Size())
	assert.Greater(t, int64(10*diskutils.MiB), hashStat.Size())
	assert.Greater(t, int64(150*diskutils.MiB), varStat.Size())

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

	bootMountPath := filepath.Join(testTempDir, "bootpartition")
	bootMount, err := safemount.NewMount(bootDevice.DevicePath(), bootMountPath, "ext4", 0, "", true)
	if !assert.NoError(t, err) {
		return
	}
	defer bootMount.Close()

	// Verify that verity is configured correctly.
	verifyVerityGrub(t, bootMountPath, rootDevice.DevicePath(), hashDevice.DevicePath(), "PARTLABEL=root",
		"PARTLABEL=roothash", "root", "rd.info", baseImageInfo, "panic-on-corruption")
}

func verifyVerityGrub(t *testing.T, bootPath string, dataDevice string, hashDevice string, dataId string, hashId string,
	verityType string, extraCommandLine string, baseImageInfo testBaseImageInfo, corruptionOption string,
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
	verifyVerityHelper(t, kernelArgsList, dataDevice, hashDevice, dataId, hashId, verityType, corruptionOption)

	// Verity extra command line args.
	recoveryCount := 0
	if baseImageInfo.Version == baseImageVersionAzl3 {
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
	corruptionOption string,
) {
	// Extract kernel command line args.
	kernelToArgs, err := extractKernelCmdlineFromUkiEfis(espPath, buildDir)
	if !assert.NoError(t, err) {
		return
	}

	// Convert map[string]string → []string
	kernelArgsList := make([]string, 0, len(kernelToArgs))
	for _, args := range kernelToArgs {
		kernelArgsList = append(kernelArgsList, args)
	}

	// Verify verity
	verifyVerityHelper(t, kernelArgsList, dataDevice, hashDevice, dataId, hashId, verityType, corruptionOption)

	// Verify extra command line
	if extraCommandLine != "" {
		for _, kernelArgs := range kernelArgsList {
			assert.Regexp(t, fmt.Sprintf(` %s( |$)`, regexp.QuoteMeta(extraCommandLine)), kernelArgs)
		}
	}
}

func verifyVerityHelper(t *testing.T, kernelArgsList []string, dataDevice string,
	hashDevice string, dataId string, hashId string, verityType string, corruptionOption string,
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
			assert.Regexp(t, fmt.Sprintf(` systemd.verity_root_options=%s( |$)`, corruptionOption), kernelArgs)

			hashRegexp = regexp.MustCompile(` roothash=([a-fA-F0-9]*) `)

		case "usr":
			assert.Regexp(t, ` rd.systemd.verity=1 `, kernelArgs)
			assert.Regexp(t, fmt.Sprintf(` systemd.verity_usr_data=%s `, dataId), kernelArgs)
			assert.Regexp(t, fmt.Sprintf(` systemd.verity_usr_hash=%s `, hashId), kernelArgs)
			assert.Regexp(t, fmt.Sprintf(` systemd.verity_usr_options=%s( |$)`, corruptionOption), kernelArgs)

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
	for _, baseImageInfo := range baseImageAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageVerityUsrHelper(t, "TestCustomizeImageVerityUsr"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImageVerityUsrHelper(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTempDir := filepath.Join(tmpDir, testName)
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	configFile := filepath.Join(testDir, "verity-usr-config.yaml")

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	verityUsrVerity(t, baseImageInfo, buildDir, outImageFilePath, "")

	// Recustomize image.
	// This helps verify that verity-enabled images can be recustomized.
	err = CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, outImageFilePath, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	verityUsrVerity(t, baseImageInfo, buildDir, outImageFilePath, "")
}

func verityUsrVerity(t *testing.T, baseImageInfo testBaseImageInfo, buildDir string,
	outImageFilePath string, corruptionOption string,
) {
	// Connect to usr verity image.
	mountPoints := []testutils.MountPoint{
		{
			PartitionNum:   4,
			Path:           "/",
			FileSystemType: "ext4",
		},
		{
			PartitionNum:   1,
			Path:           "/boot/efi",
			FileSystemType: "vfat",
		},
		{
			PartitionNum:   2,
			Path:           "/usr",
			FileSystemType: "ext4",
			Flags:          unix.MS_RDONLY,
		},
	}

	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, false /*includeDefaultMounts*/, mountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	partitions, err := getDiskPartitionsMap(imageConnection.Loopback().DevicePath())
	assert.NoError(t, err, "get disk partitions")

	// Verify that usr verity is configured correctly.
	bootPath := filepath.Join(imageConnection.Chroot().RootDir(), "/boot")
	usrDevice := testutils.PartitionDevPath(imageConnection, 2)
	hashDevice := testutils.PartitionDevPath(imageConnection, 3)
	verifyVerityGrub(t, bootPath, usrDevice, hashDevice, "UUID="+partitions[2].Uuid,
		"UUID="+partitions[3].Uuid, "usr", "rd.info", baseImageInfo, corruptionOption)
}

func TestCustomizeImageVerityUsr2Stage(t *testing.T) {
	for _, baseImageInfo := range baseImageAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageVerityUsr2StageHelper(t, "testCustomizeImageVerityUsr2Stage"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImageVerityUsr2StageHelper(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTempDir := filepath.Join(tmpDir, testName)
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	stage1ConfigFile := filepath.Join(testDir, "verity-2stage-prepare.yaml")
	stage2ConfigFile := filepath.Join(testDir, "verity-2stage-enable.yaml")
	stage3ConfigFile := filepath.Join(testDir, "verity-2stage-bad-reinit.yaml")
	stage1FilePath := filepath.Join(testTempDir, "image1.qcow2")
	stage2FilePath := filepath.Join(testTempDir, "image2.raw")
	stage3FilePath := filepath.Join(testTempDir, "image3.vhdx")

	// Stage 1: Create the partitions for verity.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, stage1ConfigFile, baseImage, nil, stage1FilePath, "qcow2",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	// Stage 2: Enable verity.
	err = CustomizeImageWithConfigFile(t.Context(), buildDir, stage2ConfigFile, stage1FilePath, nil, stage2FilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	verityUsrVerity(t, baseImageInfo, buildDir, stage2FilePath, "panic-on-corruption")

	// Stage 3: Re-apply verity settings.
	err = CustomizeImageWithConfigFile(t.Context(), buildDir, stage3ConfigFile, stage2FilePath, nil, stage3FilePath, "vhdx",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.ErrorContains(t, err, "verity (verityusr) data partition is invalid")
	assert.ErrorContains(t, err, "partition already in use as existing verity device's (usr) data partition")
}

func TestCustomizeImageVerityReinitRoot(t *testing.T) {
	for _, baseImageInfo := range baseImageAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageVerityReinitRootHelper(t, "TestCustomizeImageVerityReinitRoot"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImageVerityReinitRootHelper(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTempDir := filepath.Join(tmpDir, testName)
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	stage1ConfigFile := filepath.Join(testDir, "verity-config.yaml")
	stage2aConfigFile := filepath.Join(testDir, "verity-reinit.yaml")
	stage2bConfigFile := filepath.Join(testDir, "verity-reinit-bootloader-reset.yaml")
	stage1FilePath := filepath.Join(testTempDir, "image1.raw")
	stage2FilePath := filepath.Join(testTempDir, "image2.raw")

	// Stage 1: Initialize verity.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, stage1ConfigFile, baseImage, nil, stage1FilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	verifyRootVerity(t, baseImageInfo, buildDir, stage1FilePath)

	// Stage 2a: Reinitialize verity.
	err = CustomizeImageWithConfigFile(t.Context(), buildDir, stage2aConfigFile, stage1FilePath, nil, stage2FilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	verifyRootVerity(t, baseImageInfo, buildDir, stage1FilePath)

	// Stage 2b: Reinitialize verity + hard-reset bootloader.
	err = CustomizeImageWithConfigFile(t.Context(), buildDir, stage2bConfigFile, stage1FilePath, nil, stage2FilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	verifyRootVerity(t, baseImageInfo, buildDir, stage2FilePath)
}

func TestCustomizeImageVerityReinitUsr(t *testing.T) {
	for _, baseImageInfo := range baseImageAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageVerityReinitUsrHelper(t, "TestCustomizeImageVerityReinitUsr"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImageVerityReinitUsrHelper(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTempDir := filepath.Join(tmpDir, testName)
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	stage1ConfigFile := filepath.Join(testDir, "verity-usr-config.yaml")
	stage2ConfigFile := filepath.Join(testDir, "verity-reinit.yaml")
	stage1FilePath := filepath.Join(testTempDir, "image.raw")
	stage2FilePath := filepath.Join(testTempDir, "image.raw")

	// Stage 1: Initialize verity.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, stage1ConfigFile, baseImage, nil, stage1FilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	verityUsrVerity(t, baseImageInfo, buildDir, stage1FilePath, "")

	// Stage 2: Reinitialize verity.
	err = CustomizeImageWithConfigFile(t.Context(), buildDir, stage2ConfigFile, stage1FilePath, nil, stage2FilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	verityUsrVerity(t, baseImageInfo, buildDir, stage2FilePath, "")
}
