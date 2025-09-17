// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package main

import (
	"fmt"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
)

// distribution represents a supported Linux distribution and version combination
type distribution struct {
	name    string
	version string
}

// GetDistribution validates and returns a distribution from the CLI args
func (c *ImageCreatorCmd) getDistribution() (*distribution, error) {
	dist := &distribution{
		name:    c.Distro,
		version: c.DistroVersion,
	}
	if err := dist.validate(); err != nil {
		return nil, err
	}
	return dist, nil
}

// Validate ensures the distribution and version combination is supported
func (d *distribution) validate() error {
	// Get supported versions for this distribution

	supportedDistros := imagecustomizerapi.GetSupportedDistros()
	validVersions, exists := supportedDistros[d.name]
	if !exists {
		distros := make([]string, 0, len(supportedDistros))
		for d := range supportedDistros {
			distros = append(distros, d)
		}
		return fmt.Errorf("unsupported distribution %q. Supported distributions are: %s",
			d.name, strings.Join(distros, ", "))
	}

	// Validate version
	for _, v := range validVersions {
		if v == d.version {
			return nil
		}
	}
	return fmt.Errorf("unsupported version %q for distribution %q. Supported versions: %s",
		d.version, d.name, strings.Join(validVersions, ", "))
}
