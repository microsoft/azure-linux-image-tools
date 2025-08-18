// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/envfile"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
)

// DistroConfig represents the interface for distribution-specific configuration
type DistroConfig interface {
	// Package manager configuration
	GetPackageManagerBinary() string
	GetPackageType() string // "rpm" or "deb"
	GetReleaseVersion() string
	UsesCacheOnly() bool
	UsesInstallRoot() bool
	GetConfigFile() string

	// Distribution identification
	GetDistroName() string

	// Package source handling
	GetPackageSourceDir() string
}

// Factory function to create the appropriate distro config
func NewDistroConfig(distroName string) DistroConfig {
	switch distroName {
	case "fedora":
		return &FedoraDistroConfig{}
	case "azurelinux":
		fallthrough
	default:
		return &AzureLinuxDistroConfig{}
	}
}

// detectDistroName attempts to detect the distribution name from the chroot
func detectDistroName(imageChroot safechroot.ChrootInterface) string {
	var detectedDistro string

	// Try to detect the distro by checking for specific release files
	imageChroot.UnsafeRun(func() error {
		// First try to parse /etc/os-release for modern distro detection
		if _, err := os.Stat("/etc/os-release"); err == nil {
			fields, err := envfile.ParseEnvFile("/etc/os-release")
			if err == nil {
				distroId := fields["ID"]
				switch distroId {
				case "fedora":
					detectedDistro = "fedora"
					return nil
				case "azurelinux", "mariner":
					detectedDistro = "azurelinux"
					return nil
				default:
					// Continue to legacy detection methods
				}
			}
		}

		return fmt.Errorf("no recognizable distribution release files found")
	})

	// If detection was successful, return the detected distro
	if detectedDistro != "" {
		return detectedDistro
	}

	// Default fallback
	return "azurelinux"
}
