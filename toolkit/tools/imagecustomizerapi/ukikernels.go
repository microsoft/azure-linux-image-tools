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

func (u *UkiKernels) IsValid() error {
	if u.Auto && len(u.Kernels) > 0 {
		return fmt.Errorf("'auto' cannot coexist with a list of kernel names")
	}

	if !u.Auto && len(u.Kernels) == 0 {
		return fmt.Errorf("must specify either 'auto' or a non-empty list of kernel names")
	}

	for i, kernel := range u.Kernels {
		if err := ukiKernelVersionIsValid(kernel); err != nil {
			return fmt.Errorf("invalid kernel version at index %d:\n%w", i, err)
		}
	}

	return nil
}

func ukiKernelVersionIsValid(kernel string) error {
	if kernel == "" {
		return fmt.Errorf("empty kernel name")
	}

	versionRegex := regexp.MustCompile(`^\d+\.\d+\.\d+(\.\d+)?(-[\w\-\.]+)?$`)
	if !versionRegex.MatchString(kernel) {
		return fmt.Errorf("invalid kernel version format (%s)", kernel)
	}

	return nil
}
