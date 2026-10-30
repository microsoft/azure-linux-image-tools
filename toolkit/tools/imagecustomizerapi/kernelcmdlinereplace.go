// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type KernelCmdlineReplace struct {
	// Names of args to remove.
	RemoveArgs []string `yaml:"removeArgs" json:"removeArgs,omitempty"`

	// Args to add. These will be added at the location of the last arg removed, or at the end if no arg was removed.
	AddArgs []string `yaml:"addArgs" json:"addArgs,omitempty"`
}

func (k *KernelCmdlineReplace) IsValid() error {
	for i, arg := range k.RemoveArgs {
		if arg == "" {
			return fmt.Errorf("'removeArgs' value cannot be empty (index=%d)", i)
		}
	}

	for i, arg := range k.AddArgs {
		if arg == "" {
			return fmt.Errorf("'addArgs' value cannot be empty (index=%d)", i)
		}
	}

	return nil
}
