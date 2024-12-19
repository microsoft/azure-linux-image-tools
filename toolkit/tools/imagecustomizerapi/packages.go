// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

type Packages struct {
	UpdateExistingPackages bool     `yaml:"updateExistingPackages" json:"updateExistingPackages,omitempty"`
	InstallLists           []string `yaml:"installLists" json:"-"`
	Install                []string `yaml:"install" json:"install,omitempty"`
	RemoveLists            []string `yaml:"removeLists" json:"-"`
	Remove                 []string `yaml:"remove" json:"remove,omitempty"`
	UpdateLists            []string `yaml:"updateLists" json:"-"`
	Update                 []string `yaml:"update" json:"update,omitempty"`
}
