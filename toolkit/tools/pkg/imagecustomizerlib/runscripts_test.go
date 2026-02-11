// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/stretchr/testify/assert"
)

func TestCustomizeImageRunScripts(t *testing.T) {
	for _, baseImageInfo := range checkSkipForCustomizeDefaultImages(t) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageRunScripts(t, baseImageInfo)
		})
	}
}

func testCustomizeImageRunScripts(t *testing.T, baseImageInfo testBaseImageInfo) {
	var err error

	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestRunScripts_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	configFile := filepath.Join(testDir, "runscripts-writefiles-config.yaml")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	// Customize image.
	err = CustomizeImageWithConfigFileOptions(t.Context(), configFile, ImageCustomizerOptions{
		BuildDir:          buildDir,
		InputImageFile:    baseImage,
		OutputImageFile:   outImageFilePath,
		OutputImageFormat: "raw",
		PreviewFeatures:   baseImageInfo.PreviewFeatures,
	})
	if !assert.NoError(t, err) {
		return
	}

	// Mount the output disk image so that its contents can be checked.
	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, false, /*includeDefaultMounts*/
		baseImageInfo.MountPoints)
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
