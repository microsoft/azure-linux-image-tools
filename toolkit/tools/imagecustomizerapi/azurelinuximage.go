// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
	"regexp"
)

var (
	azureLinuxVersionRegex = regexp.MustCompile(`^(\d+\.\d+)(\.(\d+))?$`)

	// Limit Azure Linux variant name to what is permitted in a subsegment of an OCI reponsitory path.
	// See, OCI Spec v1.1.1: https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#pulling-manifests
	variantRegexp = regexp.MustCompile(`^[a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*$`)
)

type AzureLinuxImage struct {
	Version string `yaml:"version" json:"version,omitempty"`
	Variant string `yaml:"variant" json:"variant,omitempty"`
}

func (i *AzureLinuxImage) IsValid() error {
	variantValid := variantRegexp.MatchString(i.Variant)
	if !variantValid {
		return fmt.Errorf("invalid 'variant' field (value='%s')", i.Variant)
	}

	_, _, err := i.ParseVersion()
	if err != nil {
		return fmt.Errorf("invalid 'version' field:\n%w", err)
	}

	return nil
}

func (i *AzureLinuxImage) ParseVersion() (string, string, error) {
	groups := azureLinuxVersionRegex.FindStringSubmatch(i.Version)
	if groups == nil {
		return "", "", fmt.Errorf("invalid version value, expecting <MAJOR>.<MINOR>.<DATE> (value='%s')", i.Version)
	}

	majorMinor := groups[1]
	date := groups[3]

	return majorMinor, date, nil
}
