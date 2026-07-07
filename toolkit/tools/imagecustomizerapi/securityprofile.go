// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

// No API behavior back-compat, only API schema back-compat?

type SecurityProfile struct {
	SELinux         SELinuxSP         `yaml:"selinux" json:"selinux,omitempty"`
	Verity          VeritySP          `yaml:"verity" json:"verity,omitempty"`
	PackageTools    PackageToolsSP    `yaml:"packageTools" json:"packageTools,omitempty"`
	PackageManifest PackageManifestSP `yaml:"packageManifest" json:"packageManifest,omitempty"`
}

type SELinuxSP string

var (
	SELinuxSPDefault   = ""
	SELinuxSPEnforcing = "enforcing"
)

type VeritySP string

var (
	VeritySPDefault   = ""
	VeritySPUsr       = "usr"
	VeritySPRoot      = "root"
	VeritySPUsrOrRoot = "usr-or-root"
)

type PackageToolsSP string

var (
	PackageToolsSPDefault      = ""
	PackageToolsSPNotInstalled = "not-installed"
)

type PackageManifestSP string

var (
	PackageManifestSPDefault  = ""
	PackageManifestSPIncluded = "included"
)
