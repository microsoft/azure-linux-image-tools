// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldPreserveUkiAddon(t *testing.T) {
	espDir := t.TempDir()
	ukiFile := filepath.Join(espDir, UkiOutputDir, "vmlinuz-test.efi")

	assert.False(t, shouldPreserveUkiAddon(ukiFile, firstBootAddonFileName))

	templatePath := filepath.Join(espDir, firstBootAddonTemplatePath)
	require.NoError(t, os.MkdirAll(filepath.Dir(templatePath), 0o755))
	require.NoError(t, os.WriteFile(templatePath, []byte("firstboot"), 0o644))

	assert.True(t, shouldPreserveUkiAddon(ukiFile, firstBootAddonFileName))
	assert.False(t, shouldPreserveUkiAddon(ukiFile, "other.addon.efi"))
}

func TestRestoreFirstBootAddon(t *testing.T) {
	espDir := t.TempDir()
	templateContents := []byte("firstboot")
	templatePath := filepath.Join(espDir, firstBootAddonTemplatePath)
	require.NoError(t, os.MkdirAll(filepath.Dir(templatePath), 0o755))
	require.NoError(t, os.WriteFile(templatePath, templateContents, 0o644))

	kernelInfo := map[string]UkiKernelInfo{
		"vmlinuz-one": {},
		"vmlinuz-two": {},
	}
	require.NoError(t, restoreFirstBootAddon(espDir, kernelInfo))

	for kernel := range kernelInfo {
		addonPath := filepath.Join(espDir, UkiOutputDir, kernel+".efi.extra.d", firstBootAddonFileName)
		actual, err := os.ReadFile(addonPath)
		require.NoError(t, err)
		assert.Equal(t, templateContents, actual)
	}
}

func TestRestoreFirstBootAddonWithoutTemplate(t *testing.T) {
	espDir := t.TempDir()
	kernelInfo := map[string]UkiKernelInfo{"vmlinuz-test": {}}

	require.NoError(t, restoreFirstBootAddon(espDir, kernelInfo))
	_, err := os.Stat(filepath.Join(espDir, UkiOutputDir, "vmlinuz-test.efi.extra.d", firstBootAddonFileName))
	assert.ErrorIs(t, err, os.ErrNotExist)
}
