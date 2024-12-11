// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"regexp"
)

type UkiKernels struct {
	Auto    bool
	Kernels []string
}

// UnmarshalYAML enables UkiKernels to handle both shorthand "auto" and a structured list of kernel versions.
func (u *UkiKernels) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		// Handle "kernels: auto".
		if value.Value == "auto" {
			u.Auto = true
			u.Kernels = nil
			return nil
		}
		return fmt.Errorf("invalid value for 'kernels': expected 'auto' or a list of kernel names, got '%s'", value.Value)

	case yaml.SequenceNode:
		// Handle "kernels: - <kernel_version>".
		var kernels []string
		if err := value.Decode(&kernels); err != nil {
			return fmt.Errorf("failed to decode kernel list:\n%w", err)
		}
		u.Kernels = kernels
		u.Auto = false
		return nil

	default:
		// Invalid YAML structure.
		return fmt.Errorf("invalid YAML structure for 'kernels': must be either 'auto' or a list of kernel names")
	}
}

func (u UkiKernels) IsValid() error {
	if u.Auto && len(u.Kernels) > 0 {
		return fmt.Errorf("invalid uki kernels: 'auto' cannot coexist with a list of kernel names")
	}

	if !u.Auto && len(u.Kernels) == 0 {
		return fmt.Errorf("invalid uki kernels: must specify either 'auto' or a non-empty list of kernel names")
	}

	// Define a regex to validate kernel version strings. (e.g. 6.6.51.1-5)
	versionRegex := regexp.MustCompile(`^\d+\.\d+\.\d+(\.\d+)?(-[\w\-\.]+)?$`)

	for i, kernel := range u.Kernels {
		if kernel == "" {
			return fmt.Errorf("invalid uki kernels: kernel name at index %d is empty", i)
		}
		if !versionRegex.MatchString(kernel) {
			return fmt.Errorf("invalid uki kernels: kernel version at index %d - %s - does not match the expected format", i, kernel)
		}
	}

	return nil
}
