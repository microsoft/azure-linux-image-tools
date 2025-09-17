// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
	"strings"
)

const (
	DistroNameAzureLinux string = "azurelinux"
	DistroNameFedora     string = "fedora"
)

// distribution represents a supported Linux distribution and version combination
type Distribution struct {
	Name    string
	Version string
}

// Validate ensures the distribution and version combination is supported
func (d *Distribution) Validate() error {
	// Get supported versions for this distribution

	supportedDistros := getSupportedDistros()
	validVersions, exists := supportedDistros[d.Name]
	if !exists {
		distros := make([]string, 0, len(supportedDistros))
		for d := range supportedDistros {
			distros = append(distros, string(d))
		}
		return fmt.Errorf("unsupported distribution %q. Supported distributions are: %s",
			d.Name, strings.Join(distros, ", "))
	}

	// Validate version
	for _, v := range validVersions {
		if v == d.Version {
			return nil
		}
	}
	return fmt.Errorf("unsupported version %q for distribution %q. Supported versions: %s",
		d.Version, d.Name, strings.Join(validVersions, ", "))
}

func getSupportedDistros() map[string][]string {
	// supportedDistros defines valid distribution and version combinations
	return map[string][]string{
		DistroNameAzureLinux: {"2.0", "3.0"},
		DistroNameFedora:     {"42"},
	}
}
