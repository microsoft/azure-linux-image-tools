// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

// Iso defines how the generated iso media should be configured.
type Iso struct {
	KernelCommandLine KernelCommandLine  `yaml:"kernelCommandLine" json:"kernelCommandLine,omitempty"`
	AdditionalFiles   AdditionalFileList `yaml:"additionalFiles" json:"additionalFiles,omitempty"`
	InitramfsType     InitramfsImageType `yaml:"initramfsType" json:"initramfsType,omitempty"`
}

func (i *Iso) IsValid() error {
	err := i.KernelCommandLine.IsValid()
	if err != nil {
		return fmt.Errorf("invalid kernelCommandLine: %w", err)
	}

	err = i.AdditionalFiles.IsValid()
	if err != nil {
		return fmt.Errorf("invalid additionalFiles:\n%w", err)
	}

	err = i.InitramfsType.IsValid()
	if err != nil {
		return fmt.Errorf("invalid initramfs type:\n%w", err)
	}

	return nil
}
