// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
	"slices"
)

// ValidateResourceType specifies which resources to validate in the validate-config command.
type ValidateResourceType string

const (
	// ValidateResourceTypeFiles validates local files and directories.
	ValidateResourceTypeFiles ValidateResourceType = "files"
	// ValidateResourceTypeOci validates OCI artifacts.
	ValidateResourceTypeOci ValidateResourceType = "oci"
	// ValidateResourceTypeAll validates all resources (files and OCI).
	ValidateResourceTypeAll ValidateResourceType = "all"
)

var supportedValidateResourceTypes = []string{
	string(ValidateResourceTypeFiles),
	string(ValidateResourceTypeOci),
	string(ValidateResourceTypeAll),
}

func (v ValidateResourceType) IsValid() error {
	if !slices.Contains(supportedValidateResourceTypes, string(v)) {
		return fmt.Errorf("invalid validate resource type (%s)", v)
	}

	return nil
}

// SupportedValidateResourceTypes returns all valid resource types for validation.
func SupportedValidateResourceTypes() []string {
	return supportedValidateResourceTypes
}
