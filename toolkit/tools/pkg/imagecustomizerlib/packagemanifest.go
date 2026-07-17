// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/cosiapi"
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

func readPackagesFromManifestForCosi(imageChroot safechroot.ChrootInterface) ([]cosiapi.OsPackage, error) {
	manifest, err := readPackageManifest(imageChroot)
	if err != nil {
		return nil, fmt.Errorf("%w:\n%w", ErrReadPackageManifest, err)
	}

	packages, err := manifestToCosiPackages(manifest)
	if err != nil {
		return nil, fmt.Errorf("%w:\n%w", ErrReadPackageManifest, err)
	}

	return packages, nil
}

func manifestToCosiPackages(manifest packagemanifestapi.PackageManifest) ([]cosiapi.OsPackage, error) {
	var err error

	cosiPackages := []cosiapi.OsPackage(nil)
	for _, manifestPackage := range manifest.Packages {
		version := ""
		release := ""

		switch manifestPackage.Type {
		case packagemanifestapi.PackageTypeDeb:
			version = manifestPackage.Version
			release = ""

		case packagemanifestapi.PackageTypeRpm:
			_, version, release, err = rpmParseEvr(manifestPackage.Version)
			if err != nil {
				return nil, err
			}

		default:
			return nil, fmt.Errorf("%w (name='%s', type='%s')", ErrUnknownPackageManifestType, manifestPackage.Name,
				manifestPackage.Type)
		}

		cosiPackage := cosiapi.OsPackage{
			Name:    manifestPackage.Name,
			Arch:    manifestPackage.Arch,
			Version: version,
			Release: release,
		}

		cosiPackages = append(cosiPackages, cosiPackage)
	}

	return cosiPackages, nil
}

func readPackageManifest(imageChroot safechroot.ChrootInterface) (packagemanifestapi.PackageManifest, error) {
	manifestJson, err := os.ReadFile(filepath.Join(imageChroot.RootDir(), packagemanifestapi.PackageManifestPath))
	if err != nil {
		return packagemanifestapi.PackageManifest{}, err
	}

	var manifest packagemanifestapi.PackageManifest
	err = json.Unmarshal(manifestJson, &manifest)
	if err != nil {
		return packagemanifestapi.PackageManifest{}, nil
	}

	return manifest, nil
}
