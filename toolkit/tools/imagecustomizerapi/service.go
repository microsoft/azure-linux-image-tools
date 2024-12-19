// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

func serviceNameIsValid(name string) error {
	if name == "" {
		return fmt.Errorf("name of service may not be empty")
	}

	return nil
}

type Services struct {
	Enable  []string `yaml:"enable" json:"enable,omitempty"`
	Disable []string `yaml:"disable" json:"disable,omitempty"`
}

func (s *Services) IsValid() error {
	for i, service := range s.Enable {
		if err := serviceNameIsValid(service); err != nil {
			return fmt.Errorf("invalid service enable at index (%d):\n%w", i, err)
		}
	}

	for i, service := range s.Disable {
		if err := serviceNameIsValid(service); err != nil {
			return fmt.Errorf("invalid service disable at index (%d):\n%w", i, err)
		}
	}

	return nil
}
