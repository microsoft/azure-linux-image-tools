package imagecreatorlib

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/pkg/imagecustomizerlib"
)

var (
	// ErrUnsupportedDistribution indicates an unsupported Linux distribution
	ErrUnsupportedDistribution = imagecustomizerlib.NewImageCustomizerError("Distribution:UnsupportedDistribution",
		"unsupported distro")

	// ErrUnsupportedVersion indicates an unsupported version for a given distribution
	ErrUnsupportedVersion = imagecustomizerlib.NewImageCustomizerError("Distribution:UnsupportedVersion",
		"unsupported distro-version")
)

// distribution represents a supported Linux distribution and version combination
type Distribution struct {
	Name    string
	Version string
}

// Validate ensures the distribution and version combination is supported
func (d *Distribution) Validate() error {
	// Get supported versions for this distribution

	supportedDistros := imagecustomizerapi.GetSupportedDistros()
	validVersions, exists := supportedDistros[d.Name]
	if !exists {
		// Get sorted list of supported distributions
		distros := slices.Collect(maps.Keys(supportedDistros))
		for d := range supportedDistros {
			distros = append(distros, string(d))
		}
		return fmt.Errorf("%w (%q)\nsupported distributions are: (%s)",
			ErrUnsupportedDistribution, d.Name, strings.Join(distros, ", "))
	}

	// Validate version
	for _, v := range validVersions {
		if v == d.Version {
			return nil
		}
	}
	return fmt.Errorf("%w (%q)\nsupported versions for %q distro are: (%s)",
		ErrUnsupportedVersion, d.Version, d.Name, strings.Join(validVersions, ", "))
}
