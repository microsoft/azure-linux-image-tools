// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateDefaultGrubFileKernelCommandLineArgsEmptyFile(t *testing.T) {
	content := ""

	result, err := commonUpdateDefaultGrubFileKernelCommandLineArgs(content, nil, []string{"security=selinux", "selinux=1"})
	assert.NoError(t, err)
	assert.Equal(t, "GRUB_CMDLINE_LINUX=\" security=selinux selinux=1 \"\n", result)
}

func TestUpdateDefaultGrubFileKernelCommandLineArgsEmptyVars(t *testing.T) {
	content := "GRUB_CMDLINE_LINUX=\nGRUB_CMDLINE_LINUX_DEFAULT=\"\""

	result, err := commonUpdateDefaultGrubFileKernelCommandLineArgs(content, nil, []string{"security=selinux", "selinux=1"})
	assert.NoError(t, err)
	assert.Equal(t, "GRUB_CMDLINE_LINUX=\nGRUB_CMDLINE_LINUX_DEFAULT=\" security=selinux selinux=1 \"", result)
}

func TestUpdateDefaultGrubFileKernelCommandLineArgsRemoveVarsInNormal(t *testing.T) {
	content := "GRUB_CMDLINE_LINUX=\"security=apparmor\"\nGRUB_CMDLINE_LINUX_DEFAULT=\"\""

	result, err := commonUpdateDefaultGrubFileKernelCommandLineArgs(content, []string{"security"},
		[]string{"security=selinux", "selinux=1"})
	assert.NoError(t, err)
	assert.Equal(t, "GRUB_CMDLINE_LINUX=\"security=selinux selinux=1\"\nGRUB_CMDLINE_LINUX_DEFAULT=\"\"", result)
}

func TestUpdateDefaultGrubFileKernelCommandLineArgsRemoveVarsInRecovery(t *testing.T) {
	content := "GRUB_CMDLINE_LINUX=\"\"\nGRUB_CMDLINE_LINUX_DEFAULT=\"security=apparmor\""

	result, err := commonUpdateDefaultGrubFileKernelCommandLineArgs(content, []string{"security"},
		[]string{"security=selinux", "selinux=1"})
	assert.NoError(t, err)
	assert.Equal(t, "GRUB_CMDLINE_LINUX=\"\"\nGRUB_CMDLINE_LINUX_DEFAULT=\"security=selinux selinux=1\"", result)
}

func TestUpdateDefaultGrubFileKernelCommandLineArgsRemoveVarsInBoth(t *testing.T) {
	content := "GRUB_CMDLINE_LINUX=\"security=apparmor\"\nGRUB_CMDLINE_LINUX_DEFAULT=\"selinux=0\""

	result, err := commonUpdateDefaultGrubFileKernelCommandLineArgs(content, []string{"security", "selinux"},
		[]string{"security=selinux", "selinux=1"})
	assert.NoError(t, err)
	assert.Equal(t, "GRUB_CMDLINE_LINUX=\"\"\nGRUB_CMDLINE_LINUX_DEFAULT=\"security=selinux selinux=1\"", result)
}

func TestUpdateDefaultGrubFileKernelCommandLineArgsExistingVar(t *testing.T) {
	content := `GRUB_CMDLINE_LINUX_DEFAULT="rd.auto=1"` + "\n"

	result, err := commonUpdateDefaultGrubFileKernelCommandLineArgs(content, nil, []string{"security=selinux"})
	assert.NoError(t, err)
	assert.Equal(t, "GRUB_CMDLINE_LINUX_DEFAULT=\"rd.auto=1 security=selinux \"\n", result)
}

func TestAddExtraCommandLineToDefaultGrubFileMissingVar(t *testing.T) {
	// Empty file: GRUB_CMDLINE_LINUX_DEFAULT does not exist.
	content := ""

	result, err := commonAddExtraCommandLineToDefaultGrubFile(content, "console=tty0 console=ttyS0")
	assert.NoError(t, err)
	assert.Equal(t, "GRUB_CMDLINE_LINUX_DEFAULT=\" console=tty0 console=ttyS0 \"\n", result)
}

func TestAddExtraCommandLineToDefaultGrubFileExistingVar(t *testing.T) {
	content := `GRUB_CMDLINE_LINUX_DEFAULT=" $kernelopts"` + "\n"

	result, err := commonAddExtraCommandLineToDefaultGrubFile(content, "console=tty0")
	assert.NoError(t, err)
	assert.Equal(t, "GRUB_CMDLINE_LINUX_DEFAULT=\"  console=tty0 \\$kernelopts\"\n", result)
}
