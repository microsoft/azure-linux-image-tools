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

func setSnapshotTimeInTdnfConfig(imageChroot *safechroot.Chroot, snapshotTime imagecustomizerapi.PackageSnapshotTime) error {
	if snapshotTime == "" {
		return nil
	}

	// Parse to time.Time and convert to epoch
	layout := "2006:01:02"
	date, err := time.Parse(layout, string(snapshotTime))
	if err != nil {
		return fmt.Errorf("failed to parse snapshot time: %w", err)
	}

	// Use end of day to include all packages published on that date
	date = date.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	epoch := strconv.FormatInt(date.Unix(), 10)

	tdnfConfPath := filepath.Join(imageChroot.RootDir(), "etc/tdnf/tdnf.conf")
	if _, err := os.Stat(tdnfConfPath); os.IsNotExist(err) {
		tdnfConfDir := filepath.Dir(tdnfConfPath)
		if err := os.MkdirAll(tdnfConfDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory for tdnf.conf:\n%w", err)
		}
	}

	content := []byte{}
	if _, err := os.Stat(tdnfConfPath); err == nil {
		content, err = os.ReadFile(tdnfConfPath)
		if err != nil {
			return fmt.Errorf("failed to read tdnf config file:\n%w", err)
		}
	}

	lines := strings.Split(string(content), "\n")
	found := false

	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "snapshottime=") {
			lines[i] = fmt.Sprintf("snapshottime=%s", epoch)
			found = true
			break
		}
	}

	if !found {
		lines = append(lines, fmt.Sprintf("snapshottime=%s", epoch))
	}

	updated := strings.Join(lines, "\n")
	if err := os.WriteFile(tdnfConfPath, []byte(updated), 0644); err != nil {
		return fmt.Errorf("failed to write updated config: %w", err)
	}

	return nil
}
