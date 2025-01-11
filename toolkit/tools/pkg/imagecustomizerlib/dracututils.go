// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
)

const (
	dracutConfigDir = "etc/dracut.conf.d"
)

func addDracutConfig(dracutConfigFile string, lines []string) error {
	if _, err := os.Stat(dracutConfigFile); os.IsNotExist(err) {
		// File does not exist, create and write the lines.
		err := file.WriteLines(lines, dracutConfigFile)
		if err != nil {
			return fmt.Errorf("failed to write to dracut config file (%s):\n%w", dracutConfigFile, err)
		}
	} else {
		// File exists, append the lines.
		existingLines, err := file.ReadLines(dracutConfigFile)
		if err != nil {
			return fmt.Errorf("failed to read existing dracut config file (%s):\n%w", dracutConfigFile, err)
		}

		// Avoid duplicate lines by checking if they already exist.
		existingLineSet := make(map[string]struct{})
		for _, line := range existingLines {
			existingLineSet[line] = struct{}{}
		}

		linesToAppend := []string{}
		for _, line := range lines {
			if _, exists := existingLineSet[line]; !exists {
				linesToAppend = append(linesToAppend, line)
			}
		}

		// Append only non-duplicate lines.
		if len(linesToAppend) > 0 {
			err = file.AppendLines(linesToAppend, dracutConfigFile)
			if err != nil {
				return fmt.Errorf("failed to append to dracut config file (%s):\n%w", dracutConfigFile, err)
			}
		}
	}

	return nil
}

func addDracutModuleAndDriver(dracutModuleName string, dracutDriverName string, imageChroot *safechroot.Chroot) error {
	dracutConfigFile := filepath.Join(imageChroot.RootDir(), dracutConfigDir, dracutModuleName+".conf")
	lines := []string{
		"add_dracutmodules+=\" " + dracutModuleName + " \"",
		"add_drivers+=\" " + dracutDriverName + " \"",
	}
	return addDracutConfig(dracutConfigFile, lines)
}

func addDracutModule(dracutModuleName string, imageChroot *safechroot.Chroot) error {
	dracutConfigFile := filepath.Join(imageChroot.RootDir(), dracutConfigDir, dracutModuleName+".conf")
	lines := []string{
		"add_dracutmodules+=\" " + dracutModuleName + " \"",
	}
	return addDracutConfig(dracutConfigFile, lines)
}

func addDracutDriver(dracutDriverName string, imageChroot *safechroot.Chroot) error {
	dracutConfigFile := filepath.Join(imageChroot.RootDir(), dracutConfigDir, dracutDriverName+".conf")
	lines := []string{
		"add_drivers+=\" " + dracutDriverName + " \"",
	}
	return addDracutConfig(dracutConfigFile, lines)
}
