// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type KernelCommandLine struct {
	Replace []KernelCmdlineReplace `yaml:"replace" json:"replace,omitempty"`

	// Extra kernel command line args.
	ExtraCommandLine []string `yaml:"extraCommandLine" json:"extraCommandLine,omitempty"`
}

func (k *KernelCommandLine) IsValid() error {
	for i, arg := range k.ExtraCommandLine {
		if arg == "" {
			return fmt.Errorf("kernel argument cannot be empty at index %d", i)
		}
	}

	for i, replaceInfo := range k.Replace {
		if err := replaceInfo.IsValid(); err != nil {
			return fmt.Errorf("invalid 'replace' value at index %d:\n%w", i, err)
		}
	}

	return nil
}
