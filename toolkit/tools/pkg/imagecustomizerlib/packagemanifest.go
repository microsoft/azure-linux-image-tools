// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/spdx/tools-golang/spdx"
)

const (
	PackageManifestPath = "/usr/lib/os-manifest.spdx.json"
)

var (
	ErrUnknownPackageManifestType = NewImageCustomizerError("Packages:UnknownPackageManifestType", "unknown manifest package type")
)

func writePackageManifest(manifest spdx.Document, imageChroot *safechroot.Chroot) error {
	manifestJson, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	logger.Log.Tracef("Package manifest:\n%s", manifestJson)

	manifestPath := filepath.Join(imageChroot.RootDir(), PackageManifestPath)
	err = os.WriteFile(manifestPath, manifestJson, 0o644)
	if err != nil {
		return err
	}

	return nil
}
