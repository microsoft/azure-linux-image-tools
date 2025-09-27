// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Parser for the image builder's configuration schemas.

package configuration

const (
	SELinuxPolicyDefault = "selinux-policy"
)

// KernelCommandLine holds extra command line parameters which can be
// added to the grub config file.
//   - ImaPolicy: A list of IMA policies which will be used together
//   - ExtraCommandLine: Arbitrary parameters which will be appended to the
//     end of the kernel command line
type KernelCommandLine struct {
	CGroup           CGroup      `json:"CGroup"`
	ImaPolicy        []ImaPolicy `json:"ImaPolicy"`
	SELinux          SELinux     `json:"SELinux"`
	SELinuxPolicy    string      `json:"SELinuxPolicy"`
	EnableFIPS       bool        `json:"EnableFIPS"`
	ExtraCommandLine string      `json:"ExtraCommandLine"`
}

// GetSedDelimeter returns the delimeter which should be used with sed
// to find/replace the command line strings.
func (k *KernelCommandLine) GetSedDelimeter() (delimeter string) {
	return "`"
}
