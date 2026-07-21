// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"github.com/spdx/tools-golang/spdx"
)

type osManifestPackages struct {
	Packages      []*spdx.Package
	Relationships []*spdx.Relationship
}

func (m *osManifestPackages) Filter(filterFunc func(packageInfo *spdx.Package) bool) {
	removedSpdxIds := make(map[spdx.ElementID]any)
	newPackages := []*spdx.Package(nil)
	for _, packageInfo := range m.Packages {
		keep := filterFunc(packageInfo)
		if keep {
			newPackages = append(newPackages, packageInfo)
		} else {
			removedSpdxIds[packageInfo.PackageSPDXIdentifier] = nil
		}
	}

	m.Packages = newPackages

	newRelationships := []*spdx.Relationship(nil)
	for _, relationship := range m.Relationships {
		_, aRemoved := removedSpdxIds[relationship.RefA.ElementRefID]
		_, bRemoved := removedSpdxIds[relationship.RefB.ElementRefID]

		keep := !aRemoved && !bRemoved
		if keep {
			newRelationships = append(newRelationships, relationship)
		}
	}

	m.Relationships = newRelationships
}
