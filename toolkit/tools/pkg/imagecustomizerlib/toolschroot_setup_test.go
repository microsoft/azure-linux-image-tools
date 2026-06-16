// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveToolsChrootCacheDir(t *testing.T) {
	tests := []struct {
		name    string
		options ImageCustomizerOptions
		xdg     string
		home    string
		want    string
		wantErr error
	}{
		{
			name:    "ImageCacheDir wins over env",
			options: ImageCustomizerOptions{ImageCacheDir: "/var/cache/ic"},
			xdg:     "/should/not/be/used",
			home:    "/should/not/be/used-either",
			want:    filepath.Join("/var/cache/ic", toolsChrootCacheSubdir),
		},
		{
			name: "XDG_CACHE_HOME used when ImageCacheDir empty",
			xdg:  "/tmp/xdg",
			home: "/home/should-not-be-used",
			want: filepath.Join("/tmp/xdg", defaultToolsChrootCacheNamespace, toolsChrootCacheSubdir),
		},
		{
			name: "HOME fallback when XDG empty",
			home: "/home/test",
			want: filepath.Join("/home/test", ".cache", defaultToolsChrootCacheNamespace, toolsChrootCacheSubdir),
		},
		{
			name:    "Unresolved when nothing available",
			wantErr: ErrToolsChrootCacheDirUnresolved,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CACHE_HOME", tc.xdg)
			t.Setenv("HOME", tc.home)

			got, err := resolveToolsChrootCacheDir(tc.options)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
