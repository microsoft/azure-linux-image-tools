// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
)

var ErrValidateConfigOptionsBuildDirRequiredForOci = NewImageCustomizerError(
	"Validation:ValidateConfigOptionsBuildDirRequiredForOci",
	"--build-dir is required when --validate-resources includes 'oci' or 'all'")

// ValidateConfigOptions contains options for the validate-config command.
type ValidateConfigOptions struct {
	// BuildDir is required when --validate-resources includes 'oci' or 'all', as it is used to build the notary trust
	// store for OCI signature verification when validating .input.image.azureLinux in the customize config.
	BuildDir          string
	ValidateResources imagecustomizerapi.ValidateResourceTypes
}

// IsValid validates the ValidateConfigOptions fields.
func (o *ValidateConfigOptions) IsValid() error {
	for _, resourceType := range o.ValidateResources {
		if err := resourceType.IsValid(); err != nil {
			return err
		}
	}

	if o.ValidateResources.ValidateOci() && o.BuildDir == "" {
		return fmt.Errorf("%w", ErrValidateConfigOptionsBuildDirRequiredForOci)
	}

	return nil
}
