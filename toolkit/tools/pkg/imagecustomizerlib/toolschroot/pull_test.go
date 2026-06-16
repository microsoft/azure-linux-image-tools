// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package toolschroot

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	"github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/content/memory"
)

// configCounter forces a unique config blob digest per manifest so the
// in-memory store doesn't reject duplicate descriptors.
var configCounter uint64

func pushBlob(t *testing.T, store *memory.Store, mediaType string, data []byte) ocispec.Descriptor {
	t.Helper()
	desc := ocispec.Descriptor{
		MediaType: mediaType,
		Digest:    digest.FromBytes(data),
		Size:      int64(len(data)),
	}
	require.NoError(t, store.Push(context.Background(), desc, bytes.NewReader(data)))
	return desc
}

func pushManifest(t *testing.T, store *memory.Store, layers []ocispec.Descriptor) ocispec.Descriptor {
	t.Helper()
	configBody := []byte(fmt.Sprintf(`{"id":%d}`, atomic.AddUint64(&configCounter, 1)))
	configDesc := pushBlob(t, store, ocispec.MediaTypeImageConfig, configBody)

	manifest := ocispec.Manifest{
		Versioned: ocispecs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    configDesc,
		Layers:    layers,
	}
	manifestBytes, err := json.Marshal(manifest)
	require.NoError(t, err)

	return pushBlob(t, store, ocispec.MediaTypeImageManifest, manifestBytes)
}

func gzipBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, err := gw.Write(data)
	require.NoError(t, err)
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

func TestProvisionRequiresCacheDir(t *testing.T) {
	_, err := Provision(context.Background(), targetos.TargetOsAzureLinux3, "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCacheDirRequired))
}

func TestProvisionPropagatesUnsupportedDistro(t *testing.T) {
	cacheDir := t.TempDir()
	_, err := Provision(context.Background(), targetos.TargetOsUbuntu2204, cacheDir)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnsupportedDistro))
}

func TestResolveImageManifestDescendsThroughIndex(t *testing.T) {
	ctx := context.Background()
	store := memory.New()

	layerBytes := writeTar(t, []tarEntry{
		{header: tar.Header{Name: "etc/hosts", Typeflag: tar.TypeReg, Mode: 0o644}, body: []byte("ok")},
	})
	layerDesc := pushBlob(t, store, ocispec.MediaTypeImageLayer, layerBytes)
	platformManifestDesc := pushManifest(t, store, []ocispec.Descriptor{layerDesc})
	platformManifestDesc.Platform = &ocispec.Platform{
		OS:           "linux",
		Architecture: runtime.GOARCH,
	}

	decoyArch := "ppc64le"
	if runtime.GOARCH == "ppc64le" {
		decoyArch = "s390x"
	}
	decoyDesc := pushManifest(t, store, []ocispec.Descriptor{layerDesc})
	decoyDesc.Platform = &ocispec.Platform{OS: "linux", Architecture: decoyArch}

	index := ocispec.Index{
		Versioned: ocispecs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{decoyDesc, platformManifestDesc},
	}
	indexBytes, err := json.Marshal(index)
	require.NoError(t, err)
	indexDesc := pushBlob(t, store, ocispec.MediaTypeImageIndex, indexBytes)
	require.NoError(t, store.Tag(ctx, indexDesc, "latest"))

	got, err := resolveImageManifest(ctx, store, "latest")
	require.NoError(t, err)
	assert.Equal(t, platformManifestDesc.Digest, got.Digest)
}

func TestFetchAndExtractImageAppliesLayersInOrder(t *testing.T) {
	ctx := context.Background()
	store := memory.New()

	first := writeTar(t, []tarEntry{
		{header: tar.Header{Name: "etc/", Typeflag: tar.TypeDir, Mode: 0o755}},
		{header: tar.Header{Name: "etc/version", Typeflag: tar.TypeReg, Mode: 0o644}, body: []byte("v1")},
	})
	second := writeTar(t, []tarEntry{
		{header: tar.Header{Name: "etc/version", Typeflag: tar.TypeReg, Mode: 0o644}, body: []byte("v2")},
	})

	firstDesc := pushBlob(t, store, ocispec.MediaTypeImageLayer, first)
	secondDesc := pushBlob(t, store, ocispec.MediaTypeImageLayer, second)
	manifestDesc := pushManifest(t, store, []ocispec.Descriptor{firstDesc, secondDesc})

	destDir := t.TempDir()
	require.NoError(t, fetchAndExtractImage(ctx, store, manifestDesc, destDir))

	content, err := os.ReadFile(filepath.Join(destDir, "etc/version"))
	require.NoError(t, err)
	assert.Equal(t, "v2", string(content))
}

func TestPullAndExtractToCacheRaceWithExistingDestIsRecovered(t *testing.T) {
	ctx := context.Background()
	store := memory.New()

	layer := writeTar(t, []tarEntry{
		{header: tar.Header{Name: "hello", Typeflag: tar.TypeReg, Mode: 0o644}, body: []byte("x")},
	})
	layerDesc := pushBlob(t, store, ocispec.MediaTypeImageLayer, layer)
	manifestDesc := pushManifest(t, store, []ocispec.Descriptor{layerDesc})

	cacheDir := t.TempDir()
	destDir := cachePathForDigest(cacheDir, manifestDesc)

	require.NoError(t, os.MkdirAll(destDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(destDir, "marker"), []byte("pre-existing"), 0o644))

	require.NoError(t, pullAndExtractToCache(ctx, store, manifestDesc, destDir))

	content, err := os.ReadFile(filepath.Join(destDir, "marker"))
	require.NoError(t, err)
	assert.Equal(t, "pre-existing", string(content))

	siblings, err := os.ReadDir(filepath.Dir(destDir))
	require.NoError(t, err)
	for _, s := range siblings {
		assert.NotContains(t, s.Name(), ".tmp-", "found leftover staging dir %s", s.Name())
	}
}

func TestDecompressLayerSelectsCorrectDecoder(t *testing.T) {
	plain := []byte("hello")
	compressed := gzipBytes(t, plain)

	cases := []struct {
		name      string
		mediaType string
		input     []byte
		wantBytes []byte
		wantErr   error
	}{
		{
			name:      "OCIPlainTar",
			mediaType: ocispec.MediaTypeImageLayer,
			input:     plain,
			wantBytes: plain,
		},
		{
			name:      "OCIGzipTar",
			mediaType: ocispec.MediaTypeImageLayerGzip,
			input:     compressed,
			wantBytes: plain,
		},
		{
			name:      "DockerGzipTar",
			mediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
			input:     compressed,
			wantBytes: plain,
		},
		{
			name:      "Zstd",
			mediaType: "application/vnd.oci.image.layer.v1.tar+zstd",
			wantErr:   ErrUnsupportedLayer,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader, closer, err := decompressLayer(tc.mediaType, bytes.NewReader(tc.input))
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.wantErr))
				return
			}
			require.NoError(t, err)
			if closer != nil {
				defer closer.Close()
			}
			got, err := io.ReadAll(reader)
			require.NoError(t, err)
			assert.Equal(t, tc.wantBytes, got)
		})
	}
}
