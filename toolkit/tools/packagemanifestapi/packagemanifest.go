// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package packagemanifestapi

const (
	PackageManifestPath = "/usr/lib/package-manifest"
)

type PackageManifest struct {
	ManifestVersion ManifestVersion `json:"manifestVersion,omitempty"`
	Packages        []Package       `json:"packages,omitempty"`
}
