// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"gopkg.in/ini.v1"
)

func createTempTdnfConfigWithSnapshot(imageChroot *safechroot.Chroot, snapshotTime imagecustomizerapi.PackageSnapshotTime) error {
	if snapshotTime == "" {
		return nil
	}

	str := string(snapshotTime)

	parsedTime, err := time.Parse(time.RFC3339, str)
	if err != nil {
		parsedTime, err = time.Parse("2006-01-02", str)
		if err != nil {
			return fmt.Errorf("failed to parse snapshot time: %w", err)
		}
	}

	epoch := strconv.FormatInt(parsedTime.Unix(), 10)

	tempTdnfConfPath := filepath.Join(imageChroot.RootDir(), "tmp/custom-tdnf.conf")
	baseTdnfConfPath := filepath.Join(imageChroot.RootDir(), "etc/tdnf/tdnf.conf")

	cfg := ini.Empty()
	if _, err := os.Stat(baseTdnfConfPath); err == nil {
		if err := cfg.Append(baseTdnfConfPath); err != nil {
			return fmt.Errorf("failed to parse existing tdnf.conf: %w", err)
		}
	}

	cfg.Section("").Key("snapshottime").SetValue(epoch)

	if err := os.MkdirAll(filepath.Dir(tempTdnfConfPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for custom tdnf.conf: %w", err)
	}

	if err := cfg.SaveTo(tempTdnfConfPath); err != nil {
		return fmt.Errorf("failed to write custom tdnf.conf: %w", err)
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
