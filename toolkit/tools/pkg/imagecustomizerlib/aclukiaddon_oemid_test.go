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

func TestAclClearOemIdOutsideIcAddon(t *testing.T) {
	stubPath := aclOemIdTestStubPath(t)

	buildDir := t.TempDir()
	espDir := filepath.Join(buildDir, "esp")
	ukiPath := filepath.Join(espDir, "vmlinuz-test.efi")
	addonDir := ukiPath + ".extra.d"

	// The main UKI carries no OEM id, matching ACL where the cmdline lives in addons.
	buildTestUkiPe(t, stubPath, ukiPath, "console=tty0")

	icAddonName := ukiAddonFileName("vmlinuz-test")
	icAddonPath := filepath.Join(addonDir, icAddonName)
	oemAddonPath := filepath.Join(addonDir, "oem.addon.efi")
	verityAddonPath := filepath.Join(addonDir, "verity.addon.efi")
	unrelatedAddonPath := filepath.Join(addonDir, "unrelated.addon.efi")

	// An addon that only sets the OEM id: it should be deleted outright.
	buildTestUkiPe(t, stubPath, oemAddonPath, "flatcar.oem.id=azure")
	// An addon that sets the OEM id alongside real args: it should be rebuilt without the OEM id.
	buildTestUkiPe(t, stubPath, verityAddonPath, "systemd.verity=1 coreos.oem.id=azure")
	// An addon with no OEM id: it should be left alone.
	buildTestUkiPe(t, stubPath, unrelatedAddonPath, "quiet")
	// The IC-managed addon is rewritten by the caller, so this function must not touch it.
	buildTestUkiPe(t, stubPath, icAddonPath, "flatcar.oem.id=azure console=tty1")

	icAddonBefore, err := os.ReadFile(icAddonPath)
	require.NoError(t, err)

	err = aclClearOemIdOutsideIcAddon(ukiPath, icAddonName, stubPath, buildDir)
	require.NoError(t, err)

	// The OEM-id-only addon is gone.
	_, err = os.Stat(oemAddonPath)
	assert.True(t, os.IsNotExist(err), "oem.addon.efi should have been removed")

	// The mixed addon kept its real args and lost the OEM id.
	verityCmdline, err := extractCmdlineFromSinglePE(verityAddonPath, buildDir)
	require.NoError(t, err)
	assert.Equal(t, "systemd.verity=1", verityCmdline)

	// The unrelated addon is untouched.
	unrelatedCmdline, err := extractCmdlineFromSinglePE(unrelatedAddonPath, buildDir)
	require.NoError(t, err)
	assert.Equal(t, "quiet", unrelatedCmdline)

	// The IC-managed addon is byte-for-byte untouched.
	icAddonAfter, err := os.ReadFile(icAddonPath)
	require.NoError(t, err)
	assert.Equal(t, icAddonBefore, icAddonAfter, "the IC-managed addon must be left to the caller")
}

func TestAclClearOemIdOutsideIcAddonMainUkiHasOemId(t *testing.T) {
	stubPath := aclOemIdTestStubPath(t)

	buildDir := t.TempDir()
	espDir := filepath.Join(buildDir, "esp")
	ukiPath := filepath.Join(espDir, "vmlinuz-test.efi")

	// An OEM id baked into the signed main UKI cannot be removed in modify mode.
	buildTestUkiPe(t, stubPath, ukiPath, "console=tty0 flatcar.oem.id=azure")

	err := aclClearOemIdOutsideIcAddon(ukiPath, ukiAddonFileName("vmlinuz-test"), stubPath, buildDir)
	assert.ErrorIs(t, err, ErrAclOemIdInMainUki)
	assert.ErrorContains(t, err, "uki: mode: create")
}

func TestAclClearOemIdOutsideIcAddonNoAddonDir(t *testing.T) {
	stubPath := aclOemIdTestStubPath(t)

	buildDir := t.TempDir()
	ukiPath := filepath.Join(buildDir, "esp", "vmlinuz-test.efi")
	buildTestUkiPe(t, stubPath, ukiPath, "console=tty0")

	err := aclClearOemIdOutsideIcAddon(ukiPath, ukiAddonFileName("vmlinuz-test"), stubPath, buildDir)
	assert.NoError(t, err)
}
