// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package toolschroot

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry/remote"
)

var (
	ErrCacheDirRequired = errors.New("tools chroot cache directory is required")
	ErrPullFailed       = errors.New("failed to pull tools chroot container")
	ErrUnsupportedLayer = errors.New("unsupported tools chroot container layer media type")
)

// Docker schema 2 media types not exported by ocispec.
const (
	dockerMediaTypeManifest     = "application/vnd.docker.distribution.manifest.v2+json"
	dockerMediaTypeManifestList = "application/vnd.docker.distribution.manifest.list.v2+json"
)

// Provision returns the path to a rootfs directory for target, pulled from the
// OCI image returned by Resolve and cached under cacheDir keyed by manifest digest.
// On cache hit no network access occurs.
func Provision(ctx context.Context, target targetos.TargetOs, cacheDir string) (string, error) {
	if cacheDir == "" {
		return "", ErrCacheDirRequired
	}

	ref, err := Resolve(target)
	if err != nil {
		return "", err
	}

	logger.Log.Debugf("Tools chroot: resolving container (%s)", ref)

	repo, err := remote.NewRepository(ref)
	if err != nil {
		return "", fmt.Errorf("%w: opening repository (%s):\n%w", ErrPullFailed, ref, err)
	}

	manifestDesc, err := resolveImageManifest(ctx, repo, repo.Reference.Reference)
	if err != nil {
		return "", fmt.Errorf("%w (%s):\n%w", ErrPullFailed, ref, err)
	}

	destDir := cachePathForDigest(cacheDir, manifestDesc)

	if exists, err := pathExists(destDir); err != nil {
		return "", err
	} else if exists {
		logger.Log.Debugf("Tools chroot: cached at (%s)", destDir)
		return destDir, nil
	}

	if err := pullAndExtractToCache(ctx, repo, manifestDesc, destDir); err != nil {
		return "", err
	}

	logger.Log.Infof("Tools chroot: provisioned at (%s)", destDir)
	return destDir, nil
}

func cachePathForDigest(cacheDir string, desc ocispec.Descriptor) string {
	return filepath.Join(cacheDir, string(desc.Digest.Algorithm()), desc.Digest.Encoded())
}

// resolveImageManifest descends into a multi-arch index if needed, returning a
// single-platform image manifest descriptor.
func resolveImageManifest(ctx context.Context, source oras.ReadOnlyTarget, ref string) (ocispec.Descriptor, error) {
	desc, err := oras.Resolve(ctx, source, ref, oras.DefaultResolveOptions)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("resolving reference:\n%w", err)
	}

	if !isImageIndex(desc.MediaType) {
		return desc, nil
	}

	resolveOpts := oras.DefaultResolveOptions
	resolveOpts.TargetPlatform = &ocispec.Platform{
		OS:           "linux",
		Architecture: runtime.GOARCH,
	}

	platformDesc, err := oras.Resolve(ctx, source, ref, resolveOpts)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("resolving %s/%s entry in image index:\n%w",
			resolveOpts.TargetPlatform.OS, resolveOpts.TargetPlatform.Architecture, err)
	}

	return platformDesc, nil
}

func isImageIndex(mediaType string) bool {
	switch mediaType {
	case ocispec.MediaTypeImageIndex, dockerMediaTypeManifestList:
		return true
	default:
		return false
	}
}

// pullAndExtractToCache extracts layers into a staging dir and atomically
// renames it onto destDir. If another process wins the race, the staging dir
// is discarded.
func pullAndExtractToCache(ctx context.Context, source content.ReadOnlyStorage,
	manifestDesc ocispec.Descriptor, destDir string,
) error {
	parent := filepath.Dir(destDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("%w: creating cache parent (%s):\n%w", ErrPullFailed, parent, err)
	}

	stagingDir, err := os.MkdirTemp(parent, filepath.Base(destDir)+".tmp-*")
	if err != nil {
		return fmt.Errorf("%w: creating staging dir:\n%w", ErrPullFailed, err)
	}
	keepStaging := false
	defer func() {
		if !keepStaging {
			_ = os.RemoveAll(stagingDir)
		}
	}()

	if err := fetchAndExtractImage(ctx, source, manifestDesc, stagingDir); err != nil {
		return err
	}

	if err := os.Rename(stagingDir, destDir); err != nil {
		// Another process may have populated destDir while we were extracting.
		if exists, existsErr := pathExists(destDir); existsErr == nil && exists {
			logger.Log.Debugf("Tools chroot: lost cache race; using existing (%s)", destDir)
			return nil
		}
		return fmt.Errorf("%w: promoting staging dir to (%s):\n%w", ErrPullFailed, destDir, err)
	}
	keepStaging = true
	return nil
}

// fetchAndExtractImage fetches the manifest and applies its layers in order.
func fetchAndExtractImage(ctx context.Context, source content.ReadOnlyStorage,
	manifestDesc ocispec.Descriptor, destDir string,
) error {
	manifest, err := fetchManifest(ctx, source, manifestDesc)
	if err != nil {
		return fmt.Errorf("%w: fetching manifest:\n%w", ErrPullFailed, err)
	}

	logger.Log.Debugf("Tools chroot: extracting %d layer(s)", len(manifest.Layers))

	for i, layerDesc := range manifest.Layers {
		if err := fetchAndExtractLayer(ctx, source, layerDesc, destDir); err != nil {
			return fmt.Errorf("%w: layer %d (%s):\n%w", ErrPullFailed, i, layerDesc.Digest, err)
		}
	}
	return nil
}

func fetchManifest(ctx context.Context, source content.ReadOnlyStorage,
	desc ocispec.Descriptor,
) (*ocispec.Manifest, error) {
	if !isImageManifest(desc.MediaType) {
		return nil, fmt.Errorf("unexpected manifest media type (%s)", desc.MediaType)
	}

	rc, err := source.Fetch(ctx, desc)
	if err != nil {
		return nil, fmt.Errorf("opening manifest blob:\n%w", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("reading manifest blob:\n%w", err)
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing manifest JSON:\n%w", err)
	}

	return &manifest, nil
}

func isImageManifest(mediaType string) bool {
	switch mediaType {
	case ocispec.MediaTypeImageManifest, dockerMediaTypeManifest:
		return true
	default:
		return false
	}
}

// fetchAndExtractLayer fetches one layer blob and applies it to destDir.
func fetchAndExtractLayer(ctx context.Context, source content.ReadOnlyStorage,
	layerDesc ocispec.Descriptor, destDir string,
) error {
	rc, err := source.Fetch(ctx, layerDesc)
	if err != nil {
		return fmt.Errorf("fetching layer blob:\n%w", err)
	}
	defer rc.Close()

	tarReader, closer, err := decompressLayer(layerDesc.MediaType, rc)
	if err != nil {
		return err
	}
	if closer != nil {
		defer closer.Close()
	}

	if err := extractTarLayer(tarReader, destDir); err != nil {
		return err
	}
	return nil
}

// decompressLayer returns a tar-byte reader plus an optional closer.
func decompressLayer(mediaType string, blob io.Reader) (io.Reader, io.Closer, error) {
	switch {
	case strings.Contains(mediaType, "+gzip") || strings.HasSuffix(mediaType, ".gzip"):
		gzr, err := gzip.NewReader(blob)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: gzip reader for %s:\n%w", ErrPullFailed, mediaType, err)
		}
		return gzr, gzr, nil

	case strings.Contains(mediaType, "+zstd"):
		return nil, nil, fmt.Errorf("%w: %s", ErrUnsupportedLayer, mediaType)

	default:
		return blob, nil, nil
	}
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("stat (%s):\n%w", path, err)
}
