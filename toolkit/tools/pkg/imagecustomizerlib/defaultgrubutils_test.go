// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateDefaultGrubFileKernelCommandLineArgsMissingVar(t *testing.T) {
	// Empty file: GRUB_CMDLINE_LINUX_DEFAULT does not exist.
	content := ""

	result, err := updateDefaultGrubFileKernelCommandLineArgs(content,
		defaultGrubFileVarNameCmdlineLinuxDefault,
		nil, []string{"security=selinux", "selinux=1"})
	assert.NoError(t, err)
	assert.Equal(t, "GRUB_CMDLINE_LINUX_DEFAULT=\" security=selinux selinux=1 \"\n", result)
}

func TestUpdateDefaultGrubFileKernelCommandLineArgsExistingVar(t *testing.T) {
	content := `GRUB_CMDLINE_LINUX_DEFAULT="rd.auto=1"` + "\n"

	result, err := updateDefaultGrubFileKernelCommandLineArgs(content,
		defaultGrubFileVarNameCmdlineLinuxDefault,
		nil, []string{"security=selinux"})
	assert.NoError(t, err)
	assert.Equal(t, "GRUB_CMDLINE_LINUX_DEFAULT=\"rd.auto=1 security=selinux \"\n", result)
}

func TestAddExtraCommandLineToDefaultGrubFileMissingVar(t *testing.T) {
	// Empty file: GRUB_CMDLINE_LINUX_DEFAULT does not exist.
	content := ""

	result, err := addExtraCommandLineToDefaultGrubFile(content, "console=tty0 console=ttyS0")
	assert.NoError(t, err)
	assert.Equal(t, "GRUB_CMDLINE_LINUX_DEFAULT=\" console=tty0 console=ttyS0 \"\n", result)
}

func TestAddExtraCommandLineToDefaultGrubFileExistingVar(t *testing.T) {
	content := `GRUB_CMDLINE_LINUX_DEFAULT=" $kernelopts"` + "\n"

	result, err := addExtraCommandLineToDefaultGrubFile(content, "console=tty0")
	assert.NoError(t, err)
	assert.Equal(t, "GRUB_CMDLINE_LINUX_DEFAULT=\"  console=tty0 \\$kernelopts\"\n", result)
}
