// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

const (
	DistroNameAzureLinux string = "azurelinux"
	DistroNameFedora     string = "fedora"
)

func GetSupportedDistros() map[string][]string {
	// supportedDistros defines valid distribution and version combinations
	return map[string][]string{
		DistroNameAzureLinux: {"2.0", "3.0"},
		DistroNameFedora:     {"42"},
	}
}
