// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"gopkg.in/ini.v1"
)

const customTdnfConfRelPath = "tmp/custom-tdnf.conf"

func createTempTdnfConfigWithSnapshot(imageChroot *safechroot.Chroot, snapshotTime imagecustomizerapi.PackageSnapshotTime) error {
	if snapshotTime == "" {
		return nil
	}

	parsedTime, err := snapshotTime.Parse()
	if err != nil {
		return fmt.Errorf("failed to parse snapshot time:\n%w", err)
	}

	epoch := strconv.FormatInt(parsedTime.Unix(), 10)

	tempTdnfConfPath := filepath.Join(imageChroot.RootDir(), customTdnfConfRelPath)
	baseTdnfConfPath := filepath.Join(imageChroot.RootDir(), "etc/tdnf/tdnf.conf")

	cfg := ini.Empty()
	if _, err := os.Stat(baseTdnfConfPath); err == nil {
		if err := cfg.Append(baseTdnfConfPath); err != nil {
			return fmt.Errorf("failed to parse existing tdnf.conf:\n%w", err)
		}
	}

	cfg.Section("").Key("snapshottime").SetValue(epoch)

	if err := os.MkdirAll(filepath.Dir(tempTdnfConfPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for custom tdnf.conf:\n%w", err)
	}

	if err := cfg.SaveTo(tempTdnfConfPath); err != nil {
		return fmt.Errorf("failed to write custom tdnf.conf:\n%w", err)
	}

	return nil
}

func cleanupSnapshotTimeConfig(imageChroot *safechroot.Chroot) {
	configPath := filepath.Join(imageChroot.RootDir(), customTdnfConfRelPath)

	err := os.Remove(configPath)
	if err != nil && !os.IsNotExist(err) {
		logger.Log.Warnf("failed to remove custom tdnf.conf: %v", err)
	}
}
