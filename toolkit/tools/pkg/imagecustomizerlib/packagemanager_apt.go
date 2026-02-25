// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/sirupsen/logrus"
)

// aptPackageManager implements debPackageManagerHandler for APT.
type aptPackageManager struct{}

func newAptPackageManager() *aptPackageManager {
	return &aptPackageManager{}
}

func (pm *aptPackageManager) getPackageManagerBinary() string {
	return string(packageManagerAPT)
}

func (pm *aptPackageManager) getEnvironmentVariables() []string {
	return []string{
		"DEBIAN_FRONTEND=noninteractive",
		"DEBCONF_NONINTERACTIVE_SEEN=true",
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
	}
}

// cleanPackageCache runs apt-get clean, removes APT list metadata, and truncates
// APT and dpkg log files.
func (pm *aptPackageManager) cleanPackageCache(imageChroot *safechroot.Chroot) error {
	env := append(shell.CurrentEnvironment(), pm.getEnvironmentVariables()...)

	// apt-get clean
	err := imageChroot.UnsafeRun(func() error {
		return shell.NewExecBuilder(pm.getPackageManagerBinary(), "clean").
			EnvironmentVariables(env).
			LogLevel(logrus.DebugLevel, logrus.DebugLevel).
			ErrorStderrLines(1).
			Execute()
	})
	if err != nil {
		return fmt.Errorf("failed to clean APT cache:\n%w", err)
	}

	// Remove APT lists.
	aptListsDir := filepath.Join(imageChroot.RootDir(), "var/lib/apt/lists")
	err = removeDirectoryContents(aptListsDir)
	if err != nil {
		return fmt.Errorf("failed to remove APT lists:\n%w", err)
	}

	// Truncate APT log files.
	aptLogDir := filepath.Join(imageChroot.RootDir(), "var/log/apt")
	logEntries, err := os.ReadDir(aptLogDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read APT log directory:\n%w", err)
	}

	for _, entry := range logEntries {
		if entry.IsDir() {
			continue
		}

		if filepath.Ext(entry.Name()) != ".log" {
			continue
		}

		fullPath := filepath.Join(aptLogDir, entry.Name())
		err = os.Truncate(fullPath, 0)
		if err != nil {
			return fmt.Errorf("failed to truncate log file (%s):\n%w", entry.Name(), err)
		}
	}

	// Truncate dpkg log file.
	dpkgLogPath := filepath.Join(imageChroot.RootDir(), "var/log/dpkg.log")
	err = os.Truncate(dpkgLogPath, 0)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to truncate log file (var/log/dpkg.log):\n%w", err)
	}

	return nil
}
