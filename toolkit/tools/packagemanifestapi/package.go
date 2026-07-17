// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package packagemanifestapi

type Package struct {
	Type    PackageType `json:"packages,omitempty"`
	Name    string      `json:"name,omitempty"`
	Version string      `json:"version,omitempty"`
	Arch    string      `json:"arch,omitempty"`
}
