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
		expectedLayout     UkiLayout
		expectError        bool
	}{
		{
			name: "shared layout",
			existingUkis: map[string]UkiKernelInfo{
				"vmlinuz-1": {BaseLayout: &sharedLayout, Layout: sharedLayout},
				"vmlinuz-2": {BaseLayout: &sharedLayout, Layout: sharedLayout},
			},
			expectedBaseLayout: &sharedLayout,
			expectedLayout:     sharedLayout,
		},
		{
			name: "each UKI's own Image Customizer addon is renamed for the new kernel",
			existingUkis: map[string]UkiKernelInfo{
				"vmlinuz-1": {
					BaseLayout: &UkiLayout{Addons: map[string]string{"vmlinuz-1.addon.efi": "rw"}},
					Layout:     UkiLayout{Addons: map[string]string{"vmlinuz-1.addon.efi": "ro"}},
				},
				"vmlinuz-2": {
					BaseLayout: &UkiLayout{Addons: map[string]string{"vmlinuz-2.addon.efi": "rw"}},
					Layout:     UkiLayout{Addons: map[string]string{"vmlinuz-2.addon.efi": "ro"}},
				},
			},
			expectedBaseLayout: &UkiLayout{Addons: map[string]string{"vmlinuz-3.addon.efi": "rw"}},
			expectedLayout:     UkiLayout{Addons: map[string]string{"vmlinuz-3.addon.efi": "ro"}},
		},
		{
			name: "divergent layouts",
			existingUkis: map[string]UkiKernelInfo{
				"vmlinuz-1": {
					BaseLayout: &sharedLayout,
					Layout:     sharedLayout,
				},
				"vmlinuz-2": {
					BaseLayout: &UkiLayout{MainCmdline: "root=/dev/sdb"},
					Layout:     UkiLayout{MainCmdline: "root=/dev/sdb"},
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

func TestUkiUpdateKernelCommandLineArgs(t *testing.T) {
	tests := []struct {
		Name            string
		Cmdline         string
		removeArgs      []string
		newArgs         []string
		ExpectedCmdline string
	}{
		{"Empty", "", []string{"console"}, []string{"console=tty1"}, "console=tty1"},
		{"Only", "console=ttyS0", []string{"console"}, []string{"console=tty1"}, "console=tty1"},
		{"OnlyMulti", "console=ttyS0 console=tty0", []string{"console"}, []string{"console=tty1"}, "console=tty1"},
		{"FrontMulti", "start console=ttyS0 console=tty0", []string{"console"}, []string{"console=tty1"}, "start console=tty1"},
		{"BackMulti", "console=ttyS0 console=tty0 end", []string{"console"}, []string{"console=tty1"}, "console=tty1 end"},
		{"SurroundedMulti", "start console=ttyS0 console=tty0 end", []string{"console"}, []string{"console=tty1"}, "start console=tty1 end"},
		{"NotFound", "a b", []string{"console"}, []string{"console=tty1"}, "a b console=tty1"},
		{"Interlaced", "start console=ttyS0 middle console=tty0 end", []string{"console"}, []string{"console=tty1"}, "start middle console=tty1 end"},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			newCmdline, err := ukiUpdateKernelCommandLineArgs(test.Cmdline, test.removeArgs, test.newArgs)
			assert.NoError(t, err)
			assert.Equal(t, test.ExpectedCmdline, newCmdline)
		})
	}
}

func TestUkiUpdateLayoutKernelCommandLineArgs(t *testing.T) {
	tests := []struct {
		name           string
		layout         UkiLayout
		argsToRemove   []string
		newArgs        []string
		expectedLayout UkiLayout
	}{
		{
			name: "EmptyAdd",
			layout: UkiLayout{
				MainCmdline: "",
				Addons:      map[string]string{},
			},
			argsToRemove: []string{"console"},
			newArgs:      []string{"console=tty0"},
			expectedLayout: UkiLayout{
				MainCmdline: "console=tty0",
				Addons:      map[string]string{},
			},
		},
		{
			name: "NotFound",
			layout: UkiLayout{
				MainCmdline: "a b",
				Addons: map[string]string{
					"addon1": "c d",
				},
			},
			argsToRemove: []string{"console"},
			newArgs:      []string{"console=tty0"},
			expectedLayout: UkiLayout{
				MainCmdline: "a b console=tty0",
				Addons: map[string]string{
					"addon1": "c d",
				},
			},
		},
		{
			name: "MainOnly",
			layout: UkiLayout{
				MainCmdline: "a console=tty0 b console=ttyS0 c",
				Addons:      map[string]string{},
			},
			argsToRemove: []string{"console"},
			newArgs:      []string{"console=tty0"},
			expectedLayout: UkiLayout{
				MainCmdline: "a b console=tty0 c",
				Addons:      map[string]string{},
			},
		},
		{
			name: "UnrelatedAddon",
			layout: UkiLayout{
				MainCmdline: "a console=tty0 b console=ttyS0 c",
				Addons: map[string]string{
					"addon1": "d e",
				},
			},
			argsToRemove: []string{"console"},
			newArgs:      []string{"console=tty0"},
			expectedLayout: UkiLayout{
				MainCmdline: "a b console=tty0 c",
				Addons: map[string]string{
					"addon1": "d e",
				},
			},
		},
		{
			name: "OneAddon",
			layout: UkiLayout{
				MainCmdline: "a b c",
				Addons: map[string]string{
					"addon1": "d console=ttyS0 e",
				},
			},
			argsToRemove: []string{"console"},
			newArgs:      []string{"console=tty0"},
			expectedLayout: UkiLayout{
				MainCmdline: "a b c",
				Addons: map[string]string{
					"addon1": "d console=tty0 e",
				},
			},
		},
		{
			name: "MultiMatchingAddons",
			layout: UkiLayout{
				MainCmdline: "a b c",
				Addons: map[string]string{
					"addon1": "d console=tty0 e",
					"addon2": "f console=ttyS0 g",
				},
			},
			argsToRemove: []string{"console"},
			newArgs:      []string{"console=tty0"},
			expectedLayout: UkiLayout{
				MainCmdline: "a b c",
				Addons: map[string]string{
					"addon1": "d e",
					"addon2": "f console=tty0 g",
				},
			},
		},
		{
			name: "SplitArgs",
			layout: UkiLayout{
				MainCmdline: "a arg1 b arg2 c",
				Addons: map[string]string{
					"addon1": "d arg3 e arg4 f",
					"addon2": "g arg5 h arg6 i",
				},
			},
			argsToRemove: []string{"arg1", "arg2", "arg3", "arg4", "arg5", "arg6"},
			newArgs:      []string{"arg1=1", "arg2=2", "arg3=3", "arg4=4", "arg5=5", "arg6=6", "arg7=7"},
			expectedLayout: UkiLayout{
				MainCmdline: "a b arg1=1 arg2=2 arg7=7 c",
				Addons: map[string]string{
					"addon1": "d e arg3=3 arg4=4 f",
					"addon2": "g h arg5=5 arg6=6 i",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			newLayout, err := ukiUpdateLayoutKernelCommandLineArgs(test.layout, test.argsToRemove, test.newArgs)
			assert.NoError(t, err)
			if err == nil {
				assert.Equal(t, test.expectedLayout, newLayout)
			}
		})
	}
}
