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
)

func New(distroId string, versionId string) TargetOs {
	version, _ := version.ParseBasicVersion(versionId)

	return TargetOs{
		Distro:    Distro(distroId),
		VersionId: versionId,
		Version:   version,
	}
}

func GetInstalledTargetOs(rootfs string) (TargetOs, error) {
	// Try /etc/os-release first, then fall back to /usr/lib/os-release.
	fields, err := envfile.ParseEnvFile(filepath.Join(rootfs, "etc/os-release"))
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return TargetOs{}, fmt.Errorf("failed to read /etc/os-release:\n%w", err)
		}
		fields, err = envfile.ParseEnvFile(filepath.Join(rootfs, "usr/lib/os-release"))
		if err != nil {
			return TargetOs{}, fmt.Errorf("failed to read os-release (tried /etc/os-release and /usr/lib/os-release):\n%w", err)
		}
	}

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
		return TargetOs{}, fmt.Errorf("unknown ID (%s) in os-release", distroId)
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
