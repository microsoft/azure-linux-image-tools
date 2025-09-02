// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
)

var (
	// Dracut operation errors
	ErrDracutConfigWrite  = NewImageCustomizerError("Dracut:ConfigWrite", "failed to write dracut config file")
	ErrDracutConfigRead   = NewImageCustomizerError("Dracut:ConfigRead", "failed to read dracut config file")
	ErrDracutConfigAppend = NewImageCustomizerError("Dracut:ConfigAppend", "failed to append to dracut config file")
)

const (
	dracutConfigDir = "etc/dracut.conf.d"
)

func addDracutConfig(dracutConfigFile string, lines []string) error {
	if _, err := os.Stat(dracutConfigFile); os.IsNotExist(err) {
		// File does not exist, create and write the lines.
		err := file.WriteLines(lines, dracutConfigFile)
		if err != nil {
			return fmt.Errorf("%w (file='%s'):\n%w", ErrDracutConfigWrite, dracutConfigFile, err)
		}
	} else {
		// File exists, append the lines.
		existingLines, err := file.ReadLines(dracutConfigFile)
		if err != nil {
			return fmt.Errorf("%w (file='%s'):\n%w", ErrDracutConfigRead, dracutConfigFile, err)
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
			content := strings.Join(linesToAppend, "\n") + "\n"
			err = file.Append(content, dracutConfigFile)
			if err != nil {
				return fmt.Errorf("%w (file='%s'):\n%w", ErrDracutConfigAppend, dracutConfigFile, err)
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
