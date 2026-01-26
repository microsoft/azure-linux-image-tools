// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
)

// isPackageInstalledDpkg checks if a package is installed using dpkg
func isPackageInstalledDpkg(imageChroot safechroot.ChrootInterface, packageName string) bool {
	err := imageChroot.UnsafeRun(func() error {
		_, _, err := shell.Execute("dpkg-query", "-W", "-f='${Status}'", packageName)
		return err
	})
	if err != nil {
		return false
	}
	return true
}

// getAllPackagesFromChrootDpkg retrieves all installed packages from a dpkg-based system
func getAllPackagesFromChrootDpkg(imageChroot safechroot.ChrootInterface) ([]OsPackage, error) {
	var out string
	err := imageChroot.UnsafeRun(func() error {
		var err error
		// Query format: package:arch version architecture
		out, _, err = shell.Execute(
			"dpkg-query", "-W", "-f=${Package}\t${Version}\t${Architecture}\n",
		)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get dpkg output from chroot:\n%w", err)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	var packages []OsPackage
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			return nil, fmt.Errorf("malformed dpkg line encountered while parsing installed packages for COSI: %q", line)
		}

		// For dpkg, it does not have a separate release field
		// Version contains epoch:version-release, use the whole thing as version
		packages = append(packages, OsPackage{
			Name:    parts[0],
			Version: parts[1],
			// dpkg doesn't have separate release
			Release: "",
			Arch:    parts[2],
		})
	}

	return packages, nil
}
