// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Parser for the image builder's configuration schemas.

package configuration

// SELinux sets the SELinux mode
type SELinux string

const (
	// SELinuxOff disables SELinux
	SELinuxOff SELinux = ""
	// SELinuxEnforcing sets SELinux to enforcing
	SELinuxEnforcing SELinux = "enforcing"
	// SELinuxPermissive sets SELinux to permissive
	SELinuxPermissive SELinux = "permissive"
	// SELinuxForceEnforcing both sets SELinux to enforcing, and forces it via the kernel command line
	SELinuxForceEnforcing SELinux = "force_enforcing"
)
