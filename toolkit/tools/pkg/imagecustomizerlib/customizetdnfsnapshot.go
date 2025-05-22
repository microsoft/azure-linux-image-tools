// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
)

func createTempTdnfConfigWithSnapshot(imageChroot *safechroot.Chroot, snapshotTime imagecustomizerapi.PackageSnapshotTime) error {
	if snapshotTime == "" {
		return nil
	}

	str := string(snapshotTime)

	var parsedTime time.Time
	var err error

	parsedTime, err = time.Parse(time.RFC3339, str)
	if err != nil {
		parsedTime, err = time.Parse("2006-01-02", str)
		if err != nil {
			return fmt.Errorf("failed to parse snapshot time: %w", err)
		}
	}

	epoch := strconv.FormatInt(parsedTime.Unix(), 10)

	tempTdnfConfPath := filepath.Join(imageChroot.RootDir(), "tmp/custom-tdnf.conf")
	baseTdnfConfPath := filepath.Join(imageChroot.RootDir(), "etc/tdnf/tdnf.conf")
	content := []byte{}

	if _, err := os.Stat(baseTdnfConfPath); err == nil {
		content, err = os.ReadFile(baseTdnfConfPath)
		if err != nil {
			return fmt.Errorf("failed to read existing tdnf.conf:\n%w", err)
		}
	}

	// Parse and modify/add snapshottime
	lines := strings.Split(string(content), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "snapshottime=") {
			lines[i] = fmt.Sprintf("snapshottime=%s", epoch)
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, fmt.Sprintf("snapshottime=%s", epoch))
	}

	finalContent := strings.Join(lines, "\n")
	tempDir := filepath.Dir(tempTdnfConfPath)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for custom tdnf.conf:\n%w", err)
	}
	if err := os.WriteFile(tempTdnfConfPath, []byte(finalContent), 0644); err != nil {
		return fmt.Errorf("failed to write custom tdnf.conf:\n%w", err)
	}

	return nil
}

func cleanupSnapshotTimeConfig(imageChroot *safechroot.Chroot) error {
	configPath := filepath.Join(imageChroot.RootDir(), "tmp/custom-tdnf.conf")

	err := os.Remove(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove custom tdnf.conf: %w", err)
	}

	return nil
}
