// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"gopkg.in/ini.v1"
)

var (
	// TDNF snapshot errors
	ErrTdnfSnapshotTimeParse         = NewImageCustomizerError("TdnfSnapshot:TimeParse", "failed to parse TDNF snapshot time")
	ErrTdnfConfigParse               = NewImageCustomizerError("TdnfSnapshot:ConfigParse", "failed to parse TDNF config")
	ErrTdnfTempConfigDirectoryCreate = NewImageCustomizerError("TdnfSnapshot:TempConfigDirectoryCreate", "failed to create directory for custom tdnf.conf")
	ErrTdnfConfigWrite               = NewImageCustomizerError("TdnfSnapshot:ConfigWrite", "failed to write TDNF config")
	ErrTdnfConfigCleanup             = NewImageCustomizerError("TdnfSnapshot:ConfigCleanup", "failed to clean up TDNF config")
)

const customTdnfConfRelPath = "tmp/custom-tdnf.conf"

func createTempTdnfConfigWithSnapshot(imageChroot *safechroot.Chroot, snapshotTime imagecustomizerapi.PackageSnapshotTime) error {
	if snapshotTime == "" {
		return nil
	}

	parsedTime, err := snapshotTime.Parse()
	if err != nil {
		return fmt.Errorf("%w (time='%s'):\n%w", ErrTdnfSnapshotTimeParse, snapshotTime, err)
	}

	epoch := strconv.FormatInt(parsedTime.Unix(), 10)

	tempTdnfConfPath := filepath.Join(imageChroot.RootDir(), customTdnfConfRelPath)
	baseTdnfConfPath := filepath.Join(imageChroot.RootDir(), "etc/tdnf/tdnf.conf")

	cfg := ini.Empty()
	if _, err := os.Stat(baseTdnfConfPath); err == nil {
		if err := cfg.Append(baseTdnfConfPath); err != nil {
			return fmt.Errorf("%w (path='%s'):\n%w", ErrTdnfConfigParse, baseTdnfConfPath, err)
		}
	} else {
		cfg.NewSection("main")
	}

	cfg.Section("main").Key("snapshottime").SetValue(epoch)

	if err := os.MkdirAll(filepath.Dir(tempTdnfConfPath), 0755); err != nil {
		return fmt.Errorf("%w (directory='%s'):\n%w", ErrTdnfTempConfigDirectoryCreate, filepath.Dir(tempTdnfConfPath), err)
	}

	if err := cfg.SaveTo(tempTdnfConfPath); err != nil {
		return fmt.Errorf("%w (path='%s'):\n%w", ErrTdnfConfigWrite, tempTdnfConfPath, err)
	}

	return nil
}

func cleanupSnapshotTimeConfig(imageChroot *safechroot.Chroot) error {
	// e.g., remove the temp config file
	err := os.Remove(filepath.Join(imageChroot.RootDir(), customTdnfConfRelPath))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("%w:\n%w", ErrTdnfConfigCleanup, err)
	}
	return nil
}
