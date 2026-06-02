// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package targetos

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/envfile"
)

type TargetOs string

const (
	TargetOsAzureLinux2          TargetOs = "azl2"
	TargetOsAzureLinux3          TargetOs = "azl3"
	TargetOsAzureLinux4          TargetOs = "azl4"
	TargetOsAzureContainerLinux3 TargetOs = "acl3"
	TargetOsFedora42             TargetOs = "fedora42"
	TargetOsUbuntu2204           TargetOs = "ubuntu2204"
	TargetOsUbuntu2404           TargetOs = "ubuntu2404"
)

// osReleaseCandidates lists the on-rootfs paths to probe for the os-release(5) file, in preference order.
var osReleaseCandidates = []string{
	"etc/os-release",
	"usr/lib/os-release",
}

// initrdReleaseCandidates lists the in-initrd paths to probe for an os-release(5)-type file, in preference order.
//
// os-release is the canonical filename and is the only candidate present in full-OS initrds built by Image Customizer
// (which pack the source rootfs directly, in which initrd-release does not exist at all).
//
// initrd-release is the dracut-runtime variant emitted by dracut's 99base module. os-release symlinks to this file in
// such cases.
var initrdReleaseCandidates = []string{
	"etc/os-release",
	"usr/lib/os-release",
	"etc/initrd-release",
	"usr/lib/initrd-release",
}

// GetInstalledTargetOs probes the rootfs for an os-release(5) file and resolves its fields to a TargetOs.
//
// Returns an error wrapping fs.ErrNotExist when none of the candidates exist on the rootfs.
func GetInstalledTargetOs(rootfs string) (TargetOs, error) {
	var fields map[string]string
	var foundCandidate string
	for _, candidate := range osReleaseCandidates {
		path := filepath.Join(rootfs, candidate)
		parsed, err := envfile.ParseEnvFile(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return "", fmt.Errorf("failed to read (%s):\n%w", path, err)
		}
		fields = parsed
		foundCandidate = candidate
		break
	}
	if fields == nil {
		return "", fmt.Errorf("failed to find an os-release file under (%s): tried %v: %w", rootfs, osReleaseCandidates,
			fs.ErrNotExist)
	}
	return targetOsFromIds(foundCandidate, fields["ID"], fields["VERSION_ID"], fields["VARIANT_ID"])
}

// GetInitrdTargetOs probes the initrd for an os-release(5)-type file (os-release or dracut's initrd-release)
// and resolves its fields to a TargetOs.
//
// Returns an error wrapping fs.ErrNotExist when none of the candidates exist in the initrd.
func GetInitrdTargetOs(initrdPath string) (TargetOs, error) {
	content, foundPath, err := readFirstFileFromInitrd(initrdPath, initrdReleaseCandidates)
	if err != nil {
		return "", err
	}

	fields, err := envfile.ParseEnv(string(content))
	if err != nil {
		return "", fmt.Errorf("failed to parse (%s) from initrd (%s):\n%w", foundPath, initrdPath, err)
	}
	return targetOsFromIds(foundPath, fields["ID"], fields["VERSION_ID"], fields["VARIANT_ID"])
}

// targetOsFromIds maps the ID, VERSION_ID, and VARIANT_ID values of an os-release(5)-style file to a TargetOs constant.
func targetOsFromIds(sourceLabel, distroId, versionId, variantId string) (TargetOs, error) {
	switch distroId {
	case "mariner":
		switch versionId {
		case "2.0":
			return TargetOsAzureLinux2, nil

		default:
			return "", fmt.Errorf("unknown VERSION_ID (%s) for CBL-Mariner in %s", versionId, sourceLabel)
		}

	case "azurelinux":
		switch variantId {
		case "azurecontainerlinux":
			// ACL currently sets VERSION_ID to the full version string (e.g.
			// "3.0.20260421") Accept any version that starts with "3."
			if !strings.HasPrefix(versionId, "3.") {
				return "", fmt.Errorf("unknown VERSION_ID (%s) for Azure Container Linux in %s", versionId, sourceLabel)
			}
			return TargetOsAzureContainerLinux3, nil

		default:
			// Standard Azure Linux (or unknown variant — treat as standard).
			switch versionId {
			case "3.0":
				return TargetOsAzureLinux3, nil

			case "4.0":
				return TargetOsAzureLinux4, nil

			default:
				return "", fmt.Errorf("unknown VERSION_ID (%s) for Azure Linux in %s", versionId, sourceLabel)
			}
		}

	case "fedora":
		switch versionId {
		case "42":
			return TargetOsFedora42, nil

		default:
			return "", fmt.Errorf("unknown VERSION_ID (%s) for Fedora in %s", versionId, sourceLabel)
		}

	case "ubuntu":
		switch versionId {
		case "22.04":
			return TargetOsUbuntu2204, nil

		case "24.04":
			return TargetOsUbuntu2404, nil

		default:
			return "", fmt.Errorf("unknown VERSION_ID (%s) for Ubuntu in %s", versionId, sourceLabel)
		}

	default:
		return "", fmt.Errorf("unknown ID (%s) in %s", distroId, sourceLabel)
	}
}
