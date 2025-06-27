// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/randomization"
	"github.com/stretchr/testify/assert"
)

func TestAddCustomizerRelease(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test must be run as root because it uses a chroot")
	}

	proposedDir := filepath.Join(tmpDir, "TestAddCustomizerRelease")

	err := os.MkdirAll(filepath.Join(proposedDir, "etc"), os.ModePerm)
	assert.NoError(t, err)

	expectedVersion := "0.1.0"
	expectedDate := time.Now().Format(buildTimeFormat)
	_, expectedUuid, err := randomization.CreateUuid()
	assert.NoError(t, err)

	err = addCustomizerRelease(t.Context(), proposedDir, expectedVersion, expectedDate, expectedUuid)
	assert.NoError(t, err)

	releaseFilePath := filepath.Join(proposedDir, "etc/image-customizer-release")

	file, err := os.Open(releaseFilePath)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	config := make(map[string]string)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		parts := strings.Split(line, "=")
		key := parts[0]
		value := strings.Trim(parts[1], "\"")
		config[key] = value
	}

	assert.Equal(t, expectedVersion, config["TOOL_VERSION"])
	assert.Equal(t, expectedDate, config["BUILD_DATE"])
	assert.Equal(t, expectedUuid, config["IMAGE_UUID"])
}
