// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStripAclOemIdArgs(t *testing.T) {
	tests := []struct {
		name          string
		cmdline       string
		expectedArgs  []string
		expectedFound bool
	}{
		{
			name:          "no oem id",
			cmdline:       "console=tty0 quiet",
			expectedArgs:  []string{"console=tty0", "quiet"},
			expectedFound: false,
		},
		{
			name:          "flatcar spelling",
			cmdline:       "console=tty0 flatcar.oem.id=azure quiet",
			expectedArgs:  []string{"console=tty0", "quiet"},
			expectedFound: true,
		},
		{
			name:          "legacy coreos spelling",
			cmdline:       "coreos.oem.id=azure console=tty0",
			expectedArgs:  []string{"console=tty0"},
			expectedFound: true,
		},
		{
			name:          "both spellings and duplicates",
			cmdline:       "flatcar.oem.id=azure coreos.oem.id=azure quiet flatcar.oem.id=gce",
			expectedArgs:  []string{"quiet"},
			expectedFound: true,
		},
		{
			name:          "only an oem id leaves nothing",
			cmdline:       "flatcar.oem.id=azure",
			expectedArgs:  []string{},
			expectedFound: true,
		},
		{
			name:          "empty cmdline",
			cmdline:       "",
			expectedArgs:  []string{},
			expectedFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, found, err := stripAclOemIdArgs(tt.cmdline)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedArgs, args)
			assert.Equal(t, tt.expectedFound, found)
		})
	}
}

// aclOemIdTestStubPath returns the systemd addon stub used to build test addons, skipping the test
// when the host has no ukify/objcopy/stub available.
func aclOemIdTestStubPath(t *testing.T) string {
	t.Helper()

	for _, tool := range []string{"ukify", "objcopy"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available", tool)
		}
	}

	stubPath := "/usr/lib/systemd/boot/efi/addonx64.efi.stub"
	if _, err := os.Stat(stubPath); err != nil {
		t.Skipf("UKI addon stub not available (%s)", stubPath)
	}

	return stubPath
}

func buildTestUkiPe(t *testing.T, stubPath string, outputPath string, cmdline string) {
	t.Helper()

	err := os.MkdirAll(filepath.Dir(outputPath), os.ModePerm)
	require.NoError(t, err)

	err = shell.ExecuteLiveWithErr(1, "ukify", "build",
		"--cmdline="+cmdline,
		"--stub="+stubPath,
		"--output="+outputPath)
	require.NoError(t, err)
}

// kernel command line and must not be mistaken for one that sets an OEM id.
// A UKI that keeps its whole command line in addons has no .cmdline section of its own, which is how
// ACL's base ships. The absent section must resolve to "contributes nothing" and the addons must
// still be concatenated.
func TestExtractCmdlineFromUkiWithObjcopySectionlessMainUki(t *testing.T) {
	stubPath := aclOemIdTestStubPath(t)

	buildDir := t.TempDir()
	kernel := "vmlinuz-test"
	ukiPath := filepath.Join(buildDir, "esp", kernel+".efi")
	addonDir := ukiPath + ".extra.d"

	buildTestUkiPe(t, stubPath, ukiPath, "")

	hasCmdline, err := peHasSection(ukiPath, ".cmdline")
	require.NoError(t, err)
	require.False(t, hasCmdline)

	buildTestUkiPe(t, stubPath, filepath.Join(addonDir, kernel+".addon.efi"),
		"root=LABEL=ROOT flatcar.oem.id=azure")
	buildTestUkiPe(t, stubPath, filepath.Join(addonDir, "firstboot.addon.efi"), "flatcar.first_boot=detected")

	cmdline, err := extractCmdlineFromUkiWithObjcopy(ukiPath, buildDir)
	require.NoError(t, err)

	// Addons are concatenated in lexicographic order, mirroring systemd-boot.
	assert.Equal(t, "flatcar.first_boot=detected root=LABEL=ROOT flatcar.oem.id=azure", cmdline)
}
