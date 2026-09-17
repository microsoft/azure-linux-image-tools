// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Package spdxmanifest builds package manifests.
package spdxmanifest

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	spdxjson "github.com/spdx/tools-golang/json"
	spdxcommon "github.com/spdx/tools-golang/spdx/v2/common"
	spdx "github.com/spdx/tools-golang/spdx/v2/v2_2"
	spdxreader "github.com/spdx/tools-golang/spdx/v2/v2_2/rdf/reader"
)

const (
	// documentID is the required SPDX document identifier.
	documentID = "DOCUMENT"

	// rootID identifies the root package representing the OS image.
	rootID = "DocumentRoot"

	// Annex F registers purl under the PACKAGE_MANAGER category.
	referenceCategory = "PACKAGE_MANAGER"
	referenceType     = "purl"

	// Stands in for a field whose value the document creator has not determined.
	noAssertion = string(spdxreader.NOASSERTION)

	// Vendorless packages use NOASSERTION, so NTIA conformance is not guaranteed.
	supplierOrganization = "Organization"

	// The namespace need not resolve. It only has to be a unique URI for the document.
	documentNamespaceBase = "https://spdx.org/spdxdocs"
)

// An SPDXID must be an "idstring"; everything else in a package name is replaced with a dash.
var nonSpdxID = regexp.MustCompile(`[^A-Za-z0-9.\-]`)

// BuildMetadata carries the document-level values a refresh cannot derive from the package set.
type BuildMetadata struct {
	// Name determines the root package name and document namespace.
	Name string

	VersionInfo string
	ToolVersion string

	// Created must already be formatted as the spec's UTC-to-the-second timestamp.
	Created string
}

type Package struct {
	ID      string
	Name    string
	Version string
	Vendor  string
	Purl    string
}

// Build renders a package manifest.
func Build(metadata BuildMetadata, packages []Package) ([]byte, error) {
	sortedPackages := slices.Clone(packages)
	slices.SortFunc(sortedPackages, func(first Package, second Package) int {
		return strings.Compare(first.ID, second.ID)
	})

	spdxPackages := []*spdx.Package{
		{
			PackageSPDXIdentifier:   rootID,
			PackageName:             metadata.Name,
			PackageVersion:          metadata.VersionInfo,
			PackageSupplier:         &spdxcommon.Supplier{Supplier: noAssertion},
			PackageDownloadLocation: noAssertion,
			FilesAnalyzed:           false,
			PackageLicenseConcluded: noAssertion,
			PackageLicenseDeclared:  noAssertion,
			PackageCopyrightText:    noAssertion,
		},
	}

	spdxRelationships := []*spdx.Relationship{
		{
			RefA:         spdxcommon.DocElementID{ElementRefID: documentID},
			RefB:         spdxcommon.DocElementID{ElementRefID: rootID},
			Relationship: spdxcommon.TypeRelationshipDescribe,
		},
	}

	for index, pkg := range sortedPackages {
		pkgId := spdxcommon.ElementID(fmt.Sprintf("Package-%d-%s", index, nonSpdxID.ReplaceAllString(pkg.Name, "-")))

		spdxPackages = append(spdxPackages, &spdx.Package{
			PackageSPDXIdentifier:   pkgId,
			PackageName:             pkg.Name,
			PackageVersion:          pkg.Version,
			PackageSupplier:         packageSupplier(pkg),
			PackageDownloadLocation: noAssertion,
			FilesAnalyzed:           false,
			PackageLicenseConcluded: noAssertion,
			PackageLicenseDeclared:  noAssertion,
			PackageCopyrightText:    noAssertion,
			PackageExternalReferences: []*spdx.PackageExternalReference{
				{
					Category: referenceCategory,
					RefType:  referenceType,
					Locator:  pkg.Purl,
				},
			},
		})

		spdxRelationships = append(spdxRelationships, &spdx.Relationship{
			RefA:         spdxcommon.DocElementID{ElementRefID: rootID},
			RefB:         spdxcommon.DocElementID{ElementRefID: pkgId},
			Relationship: spdxcommon.TypeRelationshipContains,
		})
	}

	spdxDocument := spdx.Document{
		SPDXVersion:       spdx.Version,
		SPDXIdentifier:    documentID,
		DocumentName:      metadata.Name,
		DocumentNamespace: documentNamespace(metadata.Name, metadata.VersionInfo, sortedPackages),
		DataLicense:       spdx.DataLicense,
		CreationInfo: &spdx.CreationInfo{
			Created:  metadata.Created,
			Creators: []spdxcommon.Creator{{CreatorType: "Tool", Creator: "imagecustomizer-" + metadata.ToolVersion}},
		},
		Packages:      spdxPackages,
		Relationships: spdxRelationships,
	}

	buffer := &bytes.Buffer{}

	// Package URLs carry '&' between qualifiers, which the default HTML escaping would mangle into \u0026.
	err := spdxjson.Write(spdxDocument, buffer, spdxjson.Indent("  "), spdxjson.EscapeHTML(false))
	if err != nil {
		return nil, fmt.Errorf("failed to write package manifest:\n%w", err)
	}

	return buffer.Bytes(), nil
}

func packageSupplier(pkg Package) *spdxcommon.Supplier {
	if pkg.Vendor == noAssertion {
		logger.Log.Warnf("Clearing vendor (%s) for package (%s): Reserved SPDX keyword and cannot be used",
			pkg.Vendor, pkg.Name)
		pkg.Vendor = ""
	}

	if pkg.Vendor == "" {
		return &spdxcommon.Supplier{Supplier: noAssertion}
	}

	return &spdxcommon.Supplier{SupplierType: supplierOrganization, Supplier: pkg.Vendor}
}

// documentNamespace uses UUID version 5 (SHA-1) with the name, version, and ordered package IDs as its seed.
func documentNamespace(name string, version string, packages []Package) string {
	packageIDs := make([]string, 0, len(packages))
	for _, pkg := range packages {
		packageIDs = append(packageIDs, pkg.ID)
	}

	seed := fmt.Sprintf("%s/%s/%s", name, version, strings.Join(packageIDs, "/"))
	unique := uuid.NewSHA1(uuid.NameSpaceURL, []byte(seed))

	return fmt.Sprintf("%s/%s-%s", documentNamespaceBase, name, unique)
}
