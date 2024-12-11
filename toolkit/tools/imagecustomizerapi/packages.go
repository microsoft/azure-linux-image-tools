// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

type Packages struct {
	UpdateExistingPackages bool     `yaml:"updateExistingPackages" json:"updateExistingPackages,omitempty"`
	InstallLists           []string `yaml:"installLists" json:"installLists,omitempty"`
	Install                []string `yaml:"install" json:"install,omitempty"`
	RemoveLists            []string `yaml:"removeLists" json:"removeLists,omitempty"`
	Remove                 []string `yaml:"remove" json:"remove,omitempty"`
	UpdateLists            []string `yaml:"updateLists" json:"updateLists,omitempty"`
	Update                 []string `yaml:"update" json:"update,omitempty"`
}
