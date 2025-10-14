// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	ocifile "oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry/remote"
)

func downloadOciImage(ctx context.Context, ociImage imagecustomizerapi.OciImage, buildDir string, imageCacheDir string,
) (string, string, error) {
	if imageCacheDir == "" {
		return "", "", fmt.Errorf("image cache directory must be provided to download images")
	}

	imageCacheDirIsDir, err := file.IsDir(imageCacheDir)
	if err != nil {
		return "", "", fmt.Errorf("failed to locate image cache directory (%s):\n%w", imageCacheDir, err)
	}
	if !imageCacheDirIsDir {
		return "", "", fmt.Errorf("image cache directory is not a directory (%s):\n%w", imageCacheDir, err)
	}

	ociCacheDirectory := filepath.Join(imageCacheDir, "oci")
	cacheRepo, err := oci.NewWithContext(ctx, ociCacheDirectory)
	if err != nil {
		return "", "", fmt.Errorf("failed to open image cache directory (%s):\n%w", ociCacheDirectory, err)
	}

	remoteRepo, err := remote.NewRepository(ociImage.Uri)
	if err != nil {
		return "", "", fmt.Errorf("failed to open OCI repository (%s):\n%w", ociImage.Uri, err)
	}

	tag := remoteRepo.Reference.Reference

	ociPlatform := (*ociv1.Platform)(nil)
	if ociImage.Platform != nil {
		ociPlatform = &ociv1.Platform{
			OS:           ociImage.Platform.OS,
			Architecture: ociImage.Platform.Architecture,
		}
	} else {
		descriptor, err := oras.Resolve(ctx, remoteRepo, tag, oras.DefaultResolveOptions)
		if err != nil {
			return "", "", fmt.Errorf("failed to retrieve OCI image artifact manifest:\n%w", err)
		}

		switch descriptor.MediaType {
		case ociv1.MediaTypeImageIndex:
			// OCI is a multi-arch manifest.
			// Default to current CPU architecture.
			ociPlatform = &ociv1.Platform{
				OS:           "linux",
				Architecture: runtime.GOARCH,
			}
		}
	}

	resolveOptions := oras.DefaultResolveOptions
	resolveOptions.TargetPlatform = ociPlatform

	descriptor, err := oras.Resolve(ctx, remoteRepo, tag, resolveOptions)
	if err != nil {
		return "", "", fmt.Errorf("failed to retrieve OCI image artifact manifest:\n%w", err)
	}

	copyGraphOptions := oras.DefaultCopyGraphOptions
	copyGraphOptions.PreCopy = func(ctx context.Context, desc ociv1.Descriptor) error {
		title, hasTitle := desc.Annotations[ociv1.AnnotationTitle]
		if hasTitle {
			logger.Log.Debugf("Downloading OCI file (%s)", title)
		}

		return nil
	}

	err = oras.CopyGraph(ctx, remoteRepo, cacheRepo, descriptor, copyGraphOptions)
	if err != nil {
		return "", "", fmt.Errorf("failed to download OCI image artifact:\n%w", err)
	}

	destinationDirPath, err := os.MkdirTemp(buildDir, "oci-image-")
	if err != nil {
		return "", "", fmt.Errorf("failed to create OCI download directory:\n%w", err)
	}

	fs, err := ocifile.New(destinationDirPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to initialize OCI download directory (%s):\n%w", destinationDirPath, err)
	}
	defer fs.Close()

	copyGraphOptions.PreCopy = func(ctx context.Context, desc ociv1.Descriptor) error {
		title, hasTitle := desc.Annotations[ociv1.AnnotationTitle]
		if hasTitle {
			logger.Log.Debugf("Staging OCI file (%s)", title)
		}

		return nil
	}

	err = oras.CopyGraph(ctx, cacheRepo, fs, descriptor, copyGraphOptions)
	if err != nil {
		return "", "", fmt.Errorf("failed to stage OCI image artifact:\n%w", err)
	}

	err = fs.Close()
	if err != nil {
		return "", "", fmt.Errorf("failed to finalize OCI image download:\n%w", err)
	}

	dirEntries, err := os.ReadDir(destinationDirPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read OCI download directory:\n%w", err)
	}

	imageFilePaths := []string(nil)
	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir() {
			continue
		}

		fileExt := filepath.Ext(dirEntry.Name())
		switch fileExt {
		case ".vhdx", ".vhd", ".qcow2", ".img", ".raw":
			imageFilePaths = append(imageFilePaths, filepath.Join(destinationDirPath, dirEntry.Name()))
		}
	}

	if len(imageFilePaths) <= 0 {
		return "", "", fmt.Errorf("no image files (*.vhdx, *.vhd, *.qcow2, *.img, *.raw) found in OCI artifact")
	}

	if len(imageFilePaths) > 1 {
		err = fmt.Errorf("too many image files (*.vhdx, *.vhd, *.qcow2, *.img, *.raw) found in OCI artifact (count=%d)",
			len(imageFilePaths))
		return "", "", err
	}

	imageFilePath := imageFilePaths[0]
	return destinationDirPath, imageFilePath, nil
}
