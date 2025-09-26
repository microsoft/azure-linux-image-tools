// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Parser for the image builder's configuration schemas.

package configuration

// CGroup sets the CGroup version
type CGroup string

const (
	// CGroupDefault enables cgroupv1
	CGroupDefault CGroup = ""
	// CGroupV1 enables cgroupv1
	CGroupV1 CGroup = "version_one"
	// CGroupV2 enables cgroupv2
	CGroupV2 CGroup = "version_two"
)
