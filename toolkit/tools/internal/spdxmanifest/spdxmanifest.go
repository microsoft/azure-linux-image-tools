// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Package spdxmanifest builds package manifests.
package spdxmanifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/google/uuid"
)

const (
	spdxVersion = "SPDX-2.2"
	dataLicense = "CC0-1.0"
	documentID  = "SPDXRef-DOCUMENT"
	rootID      = "SPDXRef-DocumentRoot"

	// Annex F registers purl under the PACKAGE-MANAGER category.
	referenceCategory = "PACKAGE-MANAGER"
	referenceType     = "purl"

	describesRelationship = "DESCRIBES"
	containsRelationship  = "CONTAINS"

	// Stands in for a field whose value the document creator has not determined.
	NoAssertion = "NOASSERTION"

	// Vendorless packages use NOASSERTION, so NTIA conformance is not guaranteed.
	supplierOrganizationPrefix = "Organization: "

	// The namespace need not resolve. It only has to be a unique URI for the document.
	documentNamespaceBase = "https://azurelinux.microsoft.com/spdxdocs"
)

// An SPDXID must be an "idstring"; everything else in a package name is replaced with a dash.
var nonSpdxID = regexp.MustCompile(`[^A-Za-z0-9.\-]`)

// BuildOptions carries the document-level values a refresh cannot derive from the package set.
type BuildOptions struct {
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
func Build(options BuildOptions, packages []Package) ([]byte, error) {
	sortedPackages := slices.Clone(packages)
	slices.SortFunc(sortedPackages, func(first Package, second Package) int {
		return strings.Compare(first.ID, second.ID)
	})

	spdxPackages := []any{
		map[string]any{
			"SPDXID":           rootID,
			"name":             options.Name,
			"versionInfo":      options.VersionInfo,
			"supplier":         NoAssertion,
			"downloadLocation": NoAssertion,
			"filesAnalyzed":    false,
			"licenseConcluded": NoAssertion,
			"licenseDeclared":  NoAssertion,
			"copyrightText":    NoAssertion,
		},
	}

	spdxRelationships := []any{
		map[string]any{
			"spdxElementId":      documentID,
			"relatedSpdxElement": rootID,
			"relationshipType":   describesRelationship,
		},
	}

	for index, pkg := range sortedPackages {
		spdxID := fmt.Sprintf("SPDXRef-Package-%d-%s", index, nonSpdxID.ReplaceAllString(pkg.Name, "-"))

		spdxPackages = append(spdxPackages, map[string]any{
			"SPDXID":           spdxID,
			"name":             pkg.Name,
			"versionInfo":      pkg.Version,
			"supplier":         packageSupplier(pkg),
			"downloadLocation": NoAssertion,
			"filesAnalyzed":    false,
			"licenseConcluded": NoAssertion,
			"licenseDeclared":  NoAssertion,
			"copyrightText":    NoAssertion,
			"externalRefs": []any{
				map[string]any{
					"referenceCategory": referenceCategory,
					"referenceType":     referenceType,
					"referenceLocator":  pkg.Purl,
				},
			},
		})

		spdxRelationships = append(spdxRelationships, map[string]any{
			"spdxElementId":      rootID,
			"relatedSpdxElement": spdxID,
			"relationshipType":   containsRelationship,
		})
	}

	spdxDocument := map[string]any{
		"spdxVersion":       spdxVersion,
		"SPDXID":            documentID,
		"name":              options.Name,
		"documentNamespace": documentNamespace(options.Name, options.VersionInfo, sortedPackages),
		"documentDescribes": []any{rootID},
		"dataLicense":       dataLicense,
		"creationInfo": map[string]any{
			"created":  options.Created,
			"creators": []any{"Tool: imagecustomizer-" + options.ToolVersion},
		},
		"packages":      spdxPackages,
		"relationships": spdxRelationships,
	}

	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)

	// Package URLs carry '&' between qualifiers, which the default HTML escaping would mangle into \u0026.
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")

	err := encoder.Encode(spdxDocument)
	if err != nil {
		return nil, fmt.Errorf("failed to encode package manifest:\n%w", err)
	}

	return buffer.Bytes(), nil
}

func packageSupplier(packageInfo Package) string {
	if packageInfo.Vendor == "" {
		return NoAssertion
	}

	return supplierOrganizationPrefix + packageInfo.Vendor
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
