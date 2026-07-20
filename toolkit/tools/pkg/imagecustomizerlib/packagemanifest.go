// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/packagemanifestapi"
)

var (
	ErrUnknownPackageManifestType = NewImageCustomizerError("Packages:UnknownPackageManifestType", "unknown manifest package type")
)

func writePackageManifest(packages []packagemanifestapi.Package, imageChroot *safechroot.Chroot) error {
	manifest := packagemanifestapi.PackageManifest{
		ManifestVersion: packagemanifestapi.ManifestVersion1,
		Packages:        packages,
	}

	manifestJson, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	logger.Log.Tracef("Package manifest:\n%s", manifestJson)

	manifestPath := filepath.Join(imageChroot.RootDir(), packagemanifestapi.PackageManifestPath)
	err = os.WriteFile(manifestPath, manifestJson, 0o644)
	if err != nil {
		return err
	}

	return nil
}
