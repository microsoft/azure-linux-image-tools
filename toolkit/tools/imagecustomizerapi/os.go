// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
	"strings"

	"github.com/asaskevich/govalidator"
)

// OS defines how each system present on the image is supposed to be configured.
type OS struct {
	Hostname          string             `yaml:"hostname" json:"hostname,omitempty"`
	Packages          Packages           `yaml:"packages" json:"packages,omitempty"`
	SELinux           SELinux            `yaml:"selinux" json:"selinux,omitempty"`
	KernelCommandLine KernelCommandLine  `yaml:"kernelCommandLine" json:"kernelCommandLine,omitempty"`
	AdditionalFiles   AdditionalFileList `yaml:"additionalFiles" json:"additionalFiles,omitempty"`
	AdditionalDirs    DirConfigList      `yaml:"additionalDirs" json:"additionalDirs,omitempty"`
	Users             []User             `yaml:"users" json:"users,omitempty"`
	Services          Services           `yaml:"services" json:"services,omitempty"`
	Modules           ModuleList         `yaml:"modules" json:"modules,omitempty"`
	Overlays          *[]Overlay         `yaml:"overlays" json:"overlays,omitempty"`
	BootLoader        BootLoader         `yaml:"bootloader" json:"bootloader,omitempty"`
	Uki               *Uki               `yaml:"uki" json:"uki,omitempty"`
	ImageHistory      ImageHistory       `yaml:"imageHistory" json:"imageHistory,omitempty"`
}

func (s *OS) IsValid() error {
	var err error
	err = s.BootLoader.IsValid()
	if err != nil {
		return err
	}

	if s.Hostname != "" {
		if !govalidator.IsDNSName(s.Hostname) || strings.Contains(s.Hostname, "_") {
			return fmt.Errorf("invalid hostname (%s)", s.Hostname)
		}
	}

	err = s.ImageHistory.IsValid()
	if err != nil {
		return fmt.Errorf("invalid imageHistory:\n%w", err)
	}

	err = s.SELinux.IsValid()
	if err != nil {
		return fmt.Errorf("invalid selinux:\n%w", err)
	}

	err = s.KernelCommandLine.IsValid()
	if err != nil {
		return fmt.Errorf("invalid kernelCommandLine:\n%w", err)
	}

	err = s.AdditionalFiles.IsValid()
	if err != nil {
		return fmt.Errorf("invalid additionalFiles:\n%w", err)
	}

	err = s.AdditionalDirs.IsValid()
	if err != nil {
		return fmt.Errorf("invalid additionalDirs:\n%w", err)
	}

	for i, user := range s.Users {
		err = user.IsValid()
		if err != nil {
			return fmt.Errorf("invalid users item at index %d:\n%w", i, err)
		}
	}

	if err := s.Services.IsValid(); err != nil {
		return err
	}

	if err := s.Modules.IsValid(); err != nil {
		return err
	}

	if s.Overlays != nil {
		mountPoints := make(map[string]bool)
		upperDirs := make(map[string]bool)
		workDirs := make(map[string]bool)

		for i, overlay := range *s.Overlays {
			// Validate the overlay itself
			err := overlay.IsValid()
			if err != nil {
				return fmt.Errorf("invalid overlay at index %d:\n%w", i, err)
			}

			// Check for unique MountPoint
			if _, exists := mountPoints[overlay.MountPoint]; exists {
				return fmt.Errorf("duplicate mountPoint (%s) found in overlay at index %d", overlay.MountPoint, i)
			}
			mountPoints[overlay.MountPoint] = true

			// Check for unique UpperDir
			if _, exists := upperDirs[overlay.UpperDir]; exists {
				return fmt.Errorf("duplicate upperDir (%s) found in overlay at index %d", overlay.UpperDir, i)
			}
			upperDirs[overlay.UpperDir] = true

			// Check for unique WorkDir
			if _, exists := workDirs[overlay.WorkDir]; exists {
				return fmt.Errorf("duplicate workDir (%s) found in overlay at index %d", overlay.WorkDir, i)
			}
			workDirs[overlay.WorkDir] = true
		}
	}

	if s.Uki != nil {
		err = s.Uki.IsValid()
		if err != nil {
			return fmt.Errorf("invalid uki:\n%w", err)
		}
	}

	return nil
}
