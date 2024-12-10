// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCustomizeImageRunScripts(t *testing.T) {
	var err error

	baseImage := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageRunScripts")
	buildDir := filepath.Join(testTmpDir, "build")
	configFile := filepath.Join(testDir, "runscripts-writefiles-config.yaml")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	// Customize image.
	err = CustomizeImageWithConfigFile(buildDir, configFile, baseImage, nil, outImageFilePath, "raw", "",
		"" /*outputPXEArtifactsDir*/, false /*useBaseImageRpmRepos*/, false /*enableShrinkFilesystems*/)
	if !assert.NoError(t, err) {
		return
	}

	// Mount the output disk image so that its contents can be checked.
	imageConnection, err := connectToCoreEfiImage(buildDir, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Check the contents of the log file.
	expectedLogFileContents := `Squirrel
Working dir: /
Arg 1: panda
Arg 2: whale
ANIMAL_1: lion
ANIMAL_2: turtle
resolv.conf exists
Hyena
Working dir: /
Arg 1: duck
ANIMAL_3: african wild dog
Kangaroo
Working dir: /
Wombat
Working dir: /
Found DNS address: True
Ferret
resolv.conf exists
`

	file_contents, err := os.ReadFile(filepath.Join(imageConnection.Chroot().RootDir(), "/log.txt"))
	assert.NoError(t, err)
	assert.Equal(t, expectedLogFileContents, string(file_contents))

	// Check the file that was copied by kangaroo.sh.
	aOrigFilePath := filepath.Join(testDir, "/files/a.txt")
	aNewFilePath := filepath.Join(imageConnection.Chroot().RootDir(), "/a.txt")

	verifyFileContentsSame(t, aOrigFilePath, aNewFilePath)
}

func TestCustomizeImageRunScriptsIptables(t *testing.T) {
	var err error

	baseImage := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageRunScriptsIptables")
	buildDir := filepath.Join(testTmpDir, "build")
	configFile := filepath.Join(testDir, "runscripts-iptables.yaml")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	// Customize image.
	err = CustomizeImageWithConfigFile(buildDir, configFile, baseImage, nil, outImageFilePath, "raw", "",
		"" /*outputPXEArtifactsDir*/, false /*useBaseImageRpmRepos*/, false /*enableShrinkFilesystems*/)
	assert.ErrorContains(t, err, "failed to customize raw image")
	assert.ErrorContains(t, err, "script (postCustomization[0]) failed")
}

func TestCustomizeImageRunScriptsModprobe(t *testing.T) {
	var err error

	baseImage := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageRunScriptsModprobe")
	buildDir := filepath.Join(testTmpDir, "build")
	configFile := filepath.Join(testDir, "runscripts-modprobe.yaml")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	// Customize image.
	err = CustomizeImageWithConfigFile(buildDir, configFile, baseImage, nil, outImageFilePath, "raw", "",
		"" /*outputPXEArtifactsDir*/, false /*useBaseImageRpmRepos*/, false /*enableShrinkFilesystems*/)
	assert.ErrorContains(t, err, "failed to customize raw image")
	assert.ErrorContains(t, err, "script (postCustomization[0]) failed")
}
