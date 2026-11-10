// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFallbackUkiLayout(t *testing.T) {
	sharedLayout := UkiLayout{
		MainCmdline: "root=/dev/sda",
		Addons:      map[string]string{"oem.addon.efi": "flatcar.oem.id=azure"},
		ExtraFiles:  map[string]string{"policy.cred": "abc"},
	}

	tests := []struct {
		name               string
		existingUkis       map[string]UkiKernelInfo
		expectedBaseLayout *UkiLayout
		expectedLayout     *UkiLayout
		expectError        bool
	}{
		{
			name: "shared layout",
			existingUkis: map[string]UkiKernelInfo{
				"vmlinuz-1": {BaseLayout: &sharedLayout, Layout: &sharedLayout},
				"vmlinuz-2": {BaseLayout: &sharedLayout, Layout: &sharedLayout},
			},
			expectedBaseLayout: &sharedLayout,
			expectedLayout:     &sharedLayout,
		},
		{
			name: "each UKI's own Image Customizer addon is renamed for the new kernel",
			existingUkis: map[string]UkiKernelInfo{
				"vmlinuz-1": {
					BaseLayout: &UkiLayout{Addons: map[string]string{"vmlinuz-1.addon.efi": "rw"}},
					Layout:     &UkiLayout{Addons: map[string]string{"vmlinuz-1.addon.efi": "ro"}},
				},
				"vmlinuz-2": {
					BaseLayout: &UkiLayout{Addons: map[string]string{"vmlinuz-2.addon.efi": "rw"}},
					Layout:     &UkiLayout{Addons: map[string]string{"vmlinuz-2.addon.efi": "ro"}},
				},
			},
			expectedBaseLayout: &UkiLayout{Addons: map[string]string{"vmlinuz-3.addon.efi": "rw"}},
			expectedLayout:     &UkiLayout{Addons: map[string]string{"vmlinuz-3.addon.efi": "ro"}},
		},
		{
			name: "divergent layouts",
			existingUkis: map[string]UkiKernelInfo{
				"vmlinuz-1": {
					BaseLayout: &sharedLayout,
					Layout:     &sharedLayout,
				},
				"vmlinuz-2": {
					BaseLayout: &UkiLayout{MainCmdline: "root=/dev/sdb"},
					Layout:     &UkiLayout{MainCmdline: "root=/dev/sdb"},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseLayout, layout, layoutKernel, err := getFallbackUkiLayout(tt.existingUkis, "vmlinuz-3")
			if tt.expectError {
				assert.ErrorContains(t, err, "divergent UKI layouts")
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedBaseLayout, baseLayout)
			assert.Equal(t, tt.expectedLayout, layout)
			assert.Equal(t, "vmlinuz-1", layoutKernel)
		})
	}
}

func TestUkiLayoutChanges(t *testing.T) {
	baseLayout := &UkiLayout{
		MainCmdline: "rw",
		Addons:      map[string]string{"oem.addon.efi": "console=tty0", "slot-a.addon.efi": "usrhash=a"},
		ExtraFiles:  map[string]string{"policy.cred": "hash1"},
	}

	assert.True(t, ukiLayoutsEqual(nil, nil))
	assert.False(t, ukiLayoutsEqual(baseLayout, nil))
	assert.True(t, ukiLayoutsEqual(&UkiLayout{MainCmdline: "rw"}, &UkiLayout{MainCmdline: "rw",
		Addons: map[string]string{}, ExtraFiles: map[string]string{}}))
	assert.True(t, ukiLayoutsEqual(baseLayout, baseLayout))
	assert.Empty(t, ukiLayoutChanges(baseLayout, baseLayout))
	assert.Equal(t, []string{"main UKI command line", "added kdump.addon.efi", "changed slot-a.addon.efi",
		"removed oem.addon.efi", "changed policy.cred"},
		ukiLayoutChanges(baseLayout, &UkiLayout{
			MainCmdline: "ro",
			Addons:      map[string]string{"kdump.addon.efi": "crashkernel=256M", "slot-a.addon.efi": "usrhash=b"},
			ExtraFiles:  map[string]string{"policy.cred": "hash2"},
		}))
	assert.Equal(t, []string{"removed policy.cred"},
		ukiLayoutChanges(baseLayout, &UkiLayout{MainCmdline: "rw", Addons: baseLayout.Addons}))
}
