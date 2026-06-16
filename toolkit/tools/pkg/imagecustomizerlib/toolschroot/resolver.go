// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Package toolschroot resolves a (distro, version) pair to a public OCI
// container reference that supplies the tools chroot used for package operations.
package toolschroot

import (
	"errors"
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
)

var ErrUnsupportedDistro = errors.New("no built-in tools chroot container is registered for this distro/version")

type mapKey struct {
	distro    targetos.Distro
	versionId string
}

// ACL has no package manager in its image; AZL3 base provides the tdnf userspace.
var builtInContainerRefs = map[mapKey]string{
	{targetos.AzureLinux, "2.0"}: "mcr.microsoft.com/cbl-mariner/base/core:2.0",
	{targetos.AzureLinux, "3.0"}: "mcr.microsoft.com/azurelinux/base/core:3.0",
	// AZL4 is only published under `-beta` until GA.
	{targetos.AzureLinux, "4.0"}:          "mcr.microsoft.com/azurelinux-beta/base/core:4.0",
	{targetos.AzureContainerLinux, "3.0"}: "mcr.microsoft.com/azurelinux/base/core:3.0",
	{targetos.Fedora, "42"}:               "quay.io/fedora/fedora:42",
}

func Resolve(target targetos.TargetOs) (string, error) {
	if target.Distro == "" {
		return "", fmt.Errorf("tools chroot resolve: distro is empty: %w", ErrUnsupportedDistro)
	}
	if target.VersionId == "" {
		return "", fmt.Errorf("tools chroot resolve: version is empty for distro (%s): %w", target.Distro, ErrUnsupportedDistro)
	}

	ref, ok := builtInContainerRefs[mapKey{target.Distro, target.VersionId}]
	if !ok {
		return "", fmt.Errorf("tools chroot resolve: distro (%s) version (%s): %w", target.Distro, target.VersionId, ErrUnsupportedDistro)
	}

	return ref, nil
}
