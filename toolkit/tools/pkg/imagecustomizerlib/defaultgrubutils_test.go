// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateDefaultGrubFileKernelCommandLineArgsEmptyFile(t *testing.T) {
	content := ""

	result, err := updateDefaultGrubFileKernelCommandLineArgs(content, nil, []string{"security=selinux", "selinux=1"})
	assert.NoError(t, err)
	assert.Equal(t, "GRUB_CMDLINE_LINUX=\" security=selinux selinux=1 \"\n", result)
}

func TestUpdateDefaultGrubFileKernelCommandLineArgsEmptyVars(t *testing.T) {
	content := "GRUB_CMDLINE_LINUX=\nGRUB_CMDLINE_LINUX_DEFAULT=\"\""

	result, err := updateDefaultGrubFileKernelCommandLineArgs(content, nil, []string{"security=selinux", "selinux=1"})
	assert.NoError(t, err)
	assert.Equal(t, "GRUB_CMDLINE_LINUX=\" security=selinux selinux=1 \"\nGRUB_CMDLINE_LINUX_DEFAULT=\"\"", result)
}

func TestUpdateDefaultGrubFileKernelCommandLineArgsRemoveVarsInCommon(t *testing.T) {
	content := "GRUB_CMDLINE_LINUX=\"security=apparmor\"\nGRUB_CMDLINE_LINUX_DEFAULT=\"\""

	result, err := updateDefaultGrubFileKernelCommandLineArgs(content, []string{"security"},
		[]string{"security=selinux", "selinux=1"})
	assert.NoError(t, err)
	assert.Equal(t, "GRUB_CMDLINE_LINUX=\"security=selinux selinux=1\"\nGRUB_CMDLINE_LINUX_DEFAULT=\"\"", result)
}

func TestUpdateDefaultGrubFileKernelCommandLineArgsRemoveVarsInDefault(t *testing.T) {
	content := "GRUB_CMDLINE_LINUX=\"\"\nGRUB_CMDLINE_LINUX_DEFAULT=\"security=apparmor\""

	result, err := updateDefaultGrubFileKernelCommandLineArgs(content, []string{"security"},
		[]string{"security=selinux", "selinux=1"})
	assert.NoError(t, err)
	assert.Equal(t, "GRUB_CMDLINE_LINUX=\" security=selinux selinux=1 \"\nGRUB_CMDLINE_LINUX_DEFAULT=\"\"", result)
}

func TestUpdateDefaultGrubFileKernelCommandLineArgsRemoveVarsInRecovery(t *testing.T) {
	content := "GRUB_CMDLINE_LINUX=\"\"\nGRUB_CMDLINE_LINUX_RECOVERY=\"security=apparmor\""

	result, err := updateDefaultGrubFileKernelCommandLineArgs(content, []string{"security"},
		[]string{"security=selinux", "selinux=1"})
	assert.NoError(t, err)
	assert.Equal(t, "GRUB_CMDLINE_LINUX=\" security=selinux selinux=1 \"\nGRUB_CMDLINE_LINUX_RECOVERY=\"\"", result)
}

func TestUpdateDefaultGrubFileKernelCommandLineArgsRemoveVarsInAll(t *testing.T) {
	content := "GRUB_CMDLINE_LINUX=\"security=apparmor\"\nGRUB_CMDLINE_LINUX_DEFAULT=\"selinux=1\"\nGRUB_CMDLINE_LINUX_RECOVERY=\"selinux=0\""

	result, err := updateDefaultGrubFileKernelCommandLineArgs(content, []string{"security", "selinux"},
		[]string{"security=selinux", "selinux=1"})
	assert.NoError(t, err)
	assert.Equal(t, "GRUB_CMDLINE_LINUX=\"security=selinux selinux=1\"\nGRUB_CMDLINE_LINUX_DEFAULT=\"\"\nGRUB_CMDLINE_LINUX_RECOVERY=\"\"", result)
}

func TestUpdateDefaultGrubFileKernelCommandLineArgsExistingVar(t *testing.T) {
	content := `GRUB_CMDLINE_LINUX_DEFAULT="rd.auto=1"` + "\n"

	result, err := updateDefaultGrubFileKernelCommandLineArgs(content, nil, []string{"security=selinux"})
	assert.NoError(t, err)
	assert.Equal(t, "GRUB_CMDLINE_LINUX=\" security=selinux \"\nGRUB_CMDLINE_LINUX_DEFAULT=\"rd.auto=1\"\n", result)
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
