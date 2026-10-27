// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
	"regexp"
	"strconv"
)

var (
	// For example:
	// - 3.0
	// - 3.0.20260923
	// - 4
	// - 4-2026101604
	azureLinuxVersionRegex = regexp.MustCompile(`^(((\d+)\.(\d+)(\.(\d+))?)|((\d+)(-(\d+))?))?$`)

	// Limit Azure Linux variant name to what is permitted in a subsegment of an OCI repository path.
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

	_, err := i.GetUri()
	if err != nil {
		return fmt.Errorf("invalid 'version' field:\n%w", err)
	}

	return nil
}

func (i *AzureLinuxImage) GetUri() (string, error) {
	groups := azureLinuxVersionRegex.FindStringSubmatch(i.Version)
	if groups == nil {
		return "", fmt.Errorf("invalid version value, expecting <MAJOR>.<MINOR>[.<DATE>] or <MAJOR>[-<DATE>] (value='%s')", i.Version)
	}

	formAzl3 := groups[2]
	formAzl4 := groups[7]

	switch {
	case formAzl3 != "":
		majorStr := groups[3]
		minorStr := groups[4]
		date := groups[6]

		major, err := strconv.ParseUint(majorStr, 10, 64)
		if err != nil {
			return "", fmt.Errorf("invalid major number (%s):\n%w", majorStr, err)
		}

		if major != 2 && major != 3 {
			err = fmt.Errorf("the version format <MAJOR>.<MINOR>[.<DATE>] is only valid for AZL2 and AZL3 (version='%s')",
				i.Version)
			return "", err
		}

		tag := "latest"
		if date != "" {
			tag = i.Version
		}

		// Note: The values are already sanitized. So, there is no need to escape the values.
		uri := fmt.Sprintf("mcr.microsoft.com/azurelinux/%s.%s/image/%s:%s", majorStr, minorStr, i.Variant, tag)
		return uri, nil

	case formAzl4 != "":
		majorStr := groups[8]

		major, err := strconv.ParseUint(majorStr, 10, 64)
		if err != nil {
			return "", fmt.Errorf("invalid major number (%s):\n%w", majorStr, err)
		}

		if major < 4 {
			err = fmt.Errorf("the version format <MAJOR>[-<DATE>] is only valid for AZL4+ (version='%s')",
				i.Version)
			return "", err
		}

		// Note: The values are already sanitized. So, there is no need to escape the values.
		uri := fmt.Sprintf("mcr.microsoft.com/azurelinux/catalog/%s:%s", i.Variant, i.Version)
		return uri, nil

	default:
		return "", fmt.Errorf("internal error: bad regexp")
	}
}
