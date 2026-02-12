// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

// ValidateResourceTypes is a slice of ValidateResourceType that provides helper methods.
type ValidateResourceTypes []ValidateResourceType

// Contains checks if the given resource type is in the list, accounting for "all".
func (v ValidateResourceTypes) Contains(resourceType ValidateResourceType) bool {
	for _, t := range v {
		if t == ValidateResourceTypeAll || t == resourceType {
			return true
		}
	}
	return false
}

// ValidateFiles returns true if files should be validated.
func (v ValidateResourceTypes) ValidateFiles() bool {
	return v.Contains(ValidateResourceTypeFiles)
}

// ValidateOci returns true if OCI artifacts should be validated.
func (v ValidateResourceTypes) ValidateOci() bool {
	return v.Contains(ValidateResourceTypeOci)
}
