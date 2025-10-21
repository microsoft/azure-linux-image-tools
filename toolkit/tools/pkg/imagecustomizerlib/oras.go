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
	"oras.land/oras-go/v2/content"
	ocifile "oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
)

var (
	ErrOciDownloadMissingCacheDir = NewImageCustomizerError("Oci:MissingImageCacheDir", "image cache directory must be provided to download images")
	ErrOciDownloadCreateCacheDir  = NewImageCustomizerError("Oci:CreateCacheDir", "failed to create image cache directory")
)

func downloadOciImage(ctx context.Context, ociImage imagecustomizerapi.OciImage, buildDir string, imageCacheDir string,
) (string, error) {
	err := validateImageCacheDir(imageCacheDir)
	if err != nil {
		return "", err
	}

	remoteRepo, err := remote.NewRepository(ociImage.Uri)
	if err != nil {
		return "", fmt.Errorf("failed to open OCI repository (%s):\n%w", ociImage.Uri, err)
	}

	// remote.NewRepository() also parses the tag from the URL for us.
	tag := remoteRepo.Reference.Reference

	descriptor, err := resolveOciReference(ctx, ociImage, remoteRepo, tag)
	if err != nil {
		return "", err
	}

	digestsDir := filepath.Join(imageCacheDir, "digests", string(descriptor.Digest.Algorithm()))

	err = os.MkdirAll(digestsDir, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("%w (%s):\n%w", ErrOciDownloadCreateCacheDir, digestsDir, err)
	}

	digestDir := filepath.Join(digestsDir, string(descriptor.Digest.Encoded()))

	// Check if image has already been downloaded.
	digestDirExists, err := file.PathExists(digestDir)
	if err != nil {
		return "", fmt.Errorf("failed to check if digest cache directory exists (%s):\n%w", digestDir, err)
	}

	if !digestDirExists {
		err = downloadOciToDirectory(ctx, remoteRepo, digestDir, descriptor)
		if err != nil {
			return "", err
		}
	}

	imageFilePath, err := findImageFileInDirectory(digestDir)
	if err != nil {
		return "", err
	}

	return imageFilePath, err
}

func validateImageCacheDir(imageCacheDir string) error {
	if imageCacheDir == "" {
		return ErrOciDownloadMissingCacheDir
	}

	// Note: os.MkdirAll will error if the path is not a directory.
	err := os.MkdirAll(imageCacheDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("%w (%s):\n%w", ErrOciDownloadCreateCacheDir, imageCacheDir, err)
	}

	return nil
}

func resolveOciReference(ctx context.Context, ociImage imagecustomizerapi.OciImage, targetRepo oras.ReadOnlyTarget,
	tag string,
) (ociv1.Descriptor, error) {
	ociPlatform := (*ociv1.Platform)(nil)
	if ociImage.Platform != nil {
		ociPlatform = &ociv1.Platform{
			OS:           ociImage.Platform.OS,
			Architecture: ociImage.Platform.Architecture,
		}
	} else {
		descriptor, err := oras.Resolve(ctx, targetRepo, tag, oras.DefaultResolveOptions)
		if err != nil {
			return ociv1.Descriptor{}, fmt.Errorf("failed to retrieve OCI image artifact manifest:\n%w", err)
		}

		switch descriptor.MediaType {
		case ociv1.MediaTypeImageIndex:
			// OCI is a multi-arch manifest.
			// Default to current CPU architecture.
			ociPlatform = &ociv1.Platform{
				OS:           "linux",
				Architecture: runtime.GOARCH,
			}

		default:
			return descriptor, nil
		}
	}

	resolveOptions := oras.DefaultResolveOptions
	resolveOptions.TargetPlatform = ociPlatform

	descriptor, err := oras.Resolve(ctx, targetRepo, tag, resolveOptions)
	if err != nil {
		return ociv1.Descriptor{}, fmt.Errorf("failed to retrieve OCI image artifact manifest:\n%w", err)
	}

	return descriptor, nil
}

func downloadOciToDirectory(ctx context.Context, sourceRepo content.ReadOnlyStorage, destinationDir string,
	root ociv1.Descriptor,
) error {
	parentDir := filepath.Dir(destinationDir)
	dirName := filepath.Base(destinationDir)

	stagingDirPath, err := os.MkdirTemp(parentDir, dirName+".tmp")
	if err != nil {
		return fmt.Errorf("failed to create OCI download staging directory (%s):\n%w", stagingDirPath, err)
	}
	defer os.RemoveAll(stagingDirPath)

	fs, err := ocifile.New(stagingDirPath)
	if err != nil {
		return fmt.Errorf("failed to initialize OCI download staging directory (%s):\n%w", stagingDirPath, err)
	}
	defer fs.Close()

	copyGraphOptions := oras.DefaultCopyGraphOptions
	copyGraphOptions.PreCopy = func(ctx context.Context, desc ociv1.Descriptor) error {
		title, hasTitle := desc.Annotations[ociv1.AnnotationTitle]
		if hasTitle {
			logger.Log.Debugf("Downloading OCI file (%s)", title)
		}

		return nil
	}

	err = oras.CopyGraph(ctx, sourceRepo, fs, root, copyGraphOptions)
	if err != nil {
		return fmt.Errorf("failed to stage OCI image artifact:\n%w", err)
	}

	err = fs.Close()
	if err != nil {
		return fmt.Errorf("failed to finalize OCI image download:\n%w", err)
	}

	err = os.Rename(stagingDirPath, destinationDir)
	if err != nil {
		return fmt.Errorf("failed to rename download directory (old='%s', new='%s):\n%w", stagingDirPath,
			destinationDir, err)
	}

	return nil
}

func findImageFileInDirectory(dirPath string) (string, error) {
	dirEntries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", fmt.Errorf("failed to read OCI download directory:\n%w", err)
	}

	imageFilePaths := []string(nil)
	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir() {
			continue
		}

		fileExt := filepath.Ext(dirEntry.Name())
		switch fileExt {
		case ".vhdx", ".vhd", ".qcow2", ".img", ".raw":
			imageFilePaths = append(imageFilePaths, filepath.Join(dirPath, dirEntry.Name()))
		}
	}

	if len(imageFilePaths) <= 0 {
		return "", fmt.Errorf("no image files (*.vhdx, *.vhd, *.qcow2, *.img, *.raw) found in OCI artifact")
	}

	if len(imageFilePaths) > 1 {
		err = fmt.Errorf("too many image files (*.vhdx, *.vhd, *.qcow2, *.img, *.raw) found in OCI artifact (count=%d)",
			len(imageFilePaths))
		return "", err
	}

	imageFilePath := imageFilePaths[0]
	return imageFilePath, nil
}
