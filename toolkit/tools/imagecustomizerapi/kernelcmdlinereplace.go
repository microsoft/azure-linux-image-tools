// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
	"strings"
)

type KernelCmdlineReplace struct {
	// Names of args to remove.
	Remove []string `yaml:"remove" json:"remove,omitempty"`

	// Args to add. These will be added at the location of the last arg removed, or at the end if no arg was removed.
	Add []string `yaml:"add" json:"add,omitempty"`
}

func (k *KernelCmdlineReplace) IsValid() error {
	for i, arg := range k.Remove {
		if arg == "" {
			return fmt.Errorf("'remove' value cannot be empty (index=%d)", i)
		}

		if strings.Contains(arg, "=") {
			return fmt.Errorf("'remove' value must be a name only and may not contain a value (index=%d, arg='%s')", i,
				arg)
		}
	}

	for i, arg := range k.Add {
		if arg == "" {
			return fmt.Errorf("'add' value cannot be empty (index=%d)", i)
		}
	}

	if len(k.Remove) <= 0 && len(k.Add) <= 0 {
		return fmt.Errorf("either 'add' or 'remove' must contain at least 1 value")
	}

	return nil
}
