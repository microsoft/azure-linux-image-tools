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
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
)

func downloadOciImage(ctx context.Context, oci imagecustomizerapi.OciImage, buildDir string) (string, string, error) {
	repo, err := remote.NewRepository(oci.Uri)
	if err != nil {
		return "", "", fmt.Errorf("failed to open OCI repository (%s):\n%w", oci.Uri, err)
	}

	tag := repo.Reference.Reference

	ociPlatform := (*ociv1.Platform)(nil)
	if oci.Platform == nil {
		ociPlatform = &ociv1.Platform{
			OS:           oci.Platform.OS,
			Architecture: oci.Platform.Architecture,
		}
	} else {
		descriptor, err := oras.Resolve(ctx, repo, tag, oras.DefaultResolveOptions)
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

	copyOptions := oras.DefaultCopyOptions
	copyOptions.WithTargetPlatform(ociPlatform)
	copyOptions.PreCopy = func(ctx context.Context, desc ociv1.Descriptor) error {
		title, hasTitle := desc.Annotations[ociv1.AnnotationTitle]
		if hasTitle {
			logger.Log.Debugf("Downloading OCI file (%s)", title)
		}

		return nil
	}

	destinationDirPath, err := os.MkdirTemp(buildDir, "oci-image-")
	if err != nil {
		return "", "", fmt.Errorf("failed to create OCI download directory:\n%w", err)
	}

	fs, err := file.New(destinationDirPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to initialize OCI download directory (%s):\n%w", destinationDirPath, err)
	}
	defer fs.Close()

	_, err = oras.Copy(ctx, repo, tag, fs, tag, copyOptions)
	if err != nil {
		return "", "", fmt.Errorf("failed to download OCI image artifact:\n%w", err)
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
		return "", "", fmt.Errorf("no image files (*.vhdx, *.vhd, *.qcow2, *.img, *.raw) found in OCI artifact:\n%w", err)
	}

	if len(imageFilePaths) > 1 {
		return "", "", fmt.Errorf("too many image files (*.vhdx, *.vhd, *.qcow2, *.img, *.raw) found in OCI artifact (count=%d):\n%w", len(imageFilePaths), err)
	}

	imageFilePath := imageFilePaths[0]
	return destinationDirPath, imageFilePath, nil
}
