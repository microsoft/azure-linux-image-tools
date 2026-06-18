// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package targetos

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/envfile"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/version"
)

type Distro string

const (
	AzureLinux          Distro = "azurelinux"
	AzureContainerLinux Distro = "azurecontainerlinux"
	Fedora              Distro = "fedora"
	Ubuntu              Distro = "ubuntu"
)

type TargetOs struct {
	Distro    Distro
	VersionId string

	// Version is the parsed version of VersionId, which can be used for version comparisons.
	// Value is nil if VersionId is not a valid version string.
	Version version.Version
}

var (
	TargetOsAzureLinux2 = TargetOs{
		Distro:    AzureLinux,
		VersionId: "2.0",
		Version:   []int{2, 0},
	}

	TargetOsAzureLinux3 = TargetOs{
		Distro:    AzureLinux,
		VersionId: "3.0",
		Version:   []int{3, 0},
	}

	TargetOsAzureLinux4 = TargetOs{
		Distro:    AzureLinux,
		VersionId: "4.0",
		Version:   []int{4, 0},
	}

	TargetOsAzureContainerLinux3 = TargetOs{
		Distro:    AzureContainerLinux,
		VersionId: "3.0",
		Version:   []int{3, 0},
	}

	TargetOsFedora42 = TargetOs{
		Distro:    Fedora,
		VersionId: "42",
		Version:   []int{42},
	}

	TargetOsUbuntu2204 = TargetOs{
		Distro:    Ubuntu,
		VersionId: "22.04",
		Version:   []int{22, 4},
	}

	TargetOsUbuntu2404 = TargetOs{
		Distro:    Ubuntu,
		VersionId: "24.04",
		Version:   []int{24, 4},
	}

	// OsReleaseCandidates lists the on-rootfs paths to probe for the os-release(5) file, in preference order.
	OsReleaseFileCandidates = []string{
		"/etc/os-release",
		"/usr/lib/os-release",
	}

	// initrdReleaseCandidates lists the in-initrd paths to probe for an os-release(5)-type file, in preference order.
	//
	// os-release is the canonical filename and is the only candidate present in full-OS initrds built by Image
	// Customizer (which pack the source rootfs directly, in which initrd-release does not exist at all).
	//
	// initrd-release is the dracut-runtime variant emitted by dracut's 99base module. os-release symlinks to this file
	// in such cases, so it is a necessary fallback.
	initrdReleaseCandidates = []string{
		"/etc/os-release",
		"/usr/lib/os-release",
		"/etc/initrd-release",
		"/usr/lib/initrd-release",
	}
)

func New(distroId Distro, versionId string) TargetOs {
	version, _ := version.ParseBasicVersion(versionId)

	return TargetOs{
		Distro:    distroId,
		VersionId: versionId,
		Version:   version,
	}
}

// GetInstalledTargetOs reads the os-release(5) file from the given rootfs and resolves it to a TargetOs.
//
// Returns an error wrapping fs.ErrNotExist when none of the candidates exist on the rootfs.
func GetInstalledTargetOs(rootfs string) (TargetOs, error) {
	var err error
	var fields map[string]string

	for _, candidate := range OsReleaseFileCandidates {
		fields, err = envfile.ParseEnvFile(filepath.Join(rootfs, candidate))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return TargetOs{}, fmt.Errorf("failed to read os-release file (%s):\n%w", candidate, err)
		}

		break
	}

	if fields == nil {
		return TargetOs{}, fmt.Errorf("no os-release file found (candidates=%s):\n%w", OsReleaseFileCandidates, err)
	}

	targetOs, err := GetInstalledTargetOsFromEnvFields(fields, "os-release")
	if err != nil {
		return TargetOs{}, fmt.Errorf("failed to determine target OS from os-release file:\n%w", err)
	}

	return targetOs, nil
}

// GetInitrdTargetOs reads an os-release(5)-type file (os-release or dracut's initrd-release) from the given initrd and
// resolves it to a TargetOs.
//
// Returns an error wrapping fs.ErrNotExist when none of the candidates exist in the initrd.
func GetInitrdTargetOs(initrdPath string) (TargetOs, error) {
	content, foundPath, err := readFirstFileFromInitrd(initrdPath, initrdReleaseCandidates)
	if err != nil {
		return TargetOs{}, err
	}

	fields, err := envfile.ParseEnv(string(content))
	if err != nil {
		return TargetOs{}, fmt.Errorf("failed to read (%s) file from initrd (%s):\n%w", foundPath, initrdPath, err)
	}

	targetOs, err := GetInstalledTargetOsFromEnvFields(fields, "initrd-release")
	if err != nil {
		return TargetOs{}, fmt.Errorf("failed to determine target OS from initrd-release file:\n%w", err)
	}

	return targetOs, nil
}

func GetInstalledTargetOsFromEnvFields(fields map[string]string, sourceLabel string) (TargetOs, error) {
	distroId := fields["ID"]
	versionId := fields["VERSION_ID"]

	version, _ := version.ParseBasicVersion(versionId)

	switch distroId {
	case "mariner":
		return TargetOs{
			Distro:    AzureLinux,
			VersionId: versionId,
			Version:   version,
		}, nil

	case "azurelinux":
		variantId := fields["VARIANT_ID"]

		switch variantId {
		case "azurecontainerlinux":
			versionId = cleanAclVersionId(versionId, version)

			return TargetOs{
				Distro:    AzureContainerLinux,
				VersionId: versionId,
				Version:   version,
			}, nil

		default:
			// Standard Azure Linux (or unknown variant — treat as standard).
			return TargetOs{
				Distro:    AzureLinux,
				VersionId: versionId,
				Version:   version,
			}, nil
		}

	case "fedora":
		return TargetOs{
			Distro:    Fedora,
			VersionId: versionId,
			Version:   version,
		}, nil

	case "ubuntu":
		return TargetOs{
			Distro:    Ubuntu,
			VersionId: versionId,
			Version:   version,
		}, nil

	default:
		return TargetOs{}, fmt.Errorf("unknown ID (%s) in %s", distroId, sourceLabel)
	}
}

func cleanAclVersionId(versionId string, version version.Version) string {
	if version == nil {
		return versionId
	}

	// ACL currently sets VERSION_ID to the full version string (e.g. "3.0.20260421"), instead of using VERSION.
	// So, strip off the date to get the proper VERSION_ID.
	cleanVersion := version[:min(2, len(version))]
	versionId = cleanVersion.String()
	return versionId
}
