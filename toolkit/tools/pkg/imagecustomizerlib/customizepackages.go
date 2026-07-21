// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/spdx/tools-golang/spdx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var (
	// Package-related errors
	ErrPackageRepoMetadataRefresh       = NewImageCustomizerError("Packages:RepoMetadataRefresh", "failed to refresh repo metadata")
	ErrInvalidPackageListFile           = NewImageCustomizerError("Packages:InvalidPackageListFile", "failed to read package list file")
	ErrPackageRemove                    = NewImageCustomizerError("Packages:Remove", "failed to remove packages")
	ErrPackageAutoRemove                = NewImageCustomizerError("Packages:AutoRemove", "failed to autoremove orphaned packages")
	ErrPackageUpdate                    = NewImageCustomizerError("Packages:Update", "failed to update packages")
	ErrPackagesUpdateInstalled          = NewImageCustomizerError("Packages:UpdateInstalled", "failed to update installed packages")
	ErrPackageInstall                   = NewImageCustomizerError("Packages:Install", "failed to install packages")
	ErrPackageCacheClean                = NewImageCustomizerError("Packages:CacheClean", "failed to clean cache")
	ErrMountRpmSources                  = NewImageCustomizerError("Packages:MountRpmSources", "failed to mount RPM sources")
	ErrSnapshotTimeNotSupported         = NewImageCustomizerError("Packages:SnapshotTimeNotSupported", "snapshot time is not supported")
	ErrRemovePackageManager             = NewImageCustomizerError("Packages:RemovePackageManager", "failed to remove package manager")
	ErrRemovePackageManagerPackages     = NewImageCustomizerError("Packages:RemovePackageManagerPackages", "failed to remove package manager packages")
	ErrRemovePackageManagerFilesAndDirs = NewImageCustomizerError("Packages:RemovePackageManagerFilesAndDirs", "failed to remove package manager files and directories")
	ErrCollectManifestPackages          = NewImageCustomizerError("Packages:CollectManifestPackages", "failed to collect packages for manifest")
	ErrWritePackageManifest             = NewImageCustomizerError("Packages:WriteManifest", "failed to write package manifest")
	ErrCheckForPackageManifest          = NewImageCustomizerError("Packages:CheckForPackageManifest", "failed to check if package manifest exists")
	ErrReadPackageManifest              = NewImageCustomizerError("Packages:ReadPackageManifest", "failed to read package manifest")
)

// addRemoveAndUpdatePackages orchestrates the complete package management workflow
func addRemoveAndUpdatePackages(ctx context.Context, buildDir string, baseConfigPath string,
	config *imagecustomizerapi.OS, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	rpmsSources []string, useBaseImageRpmRepos bool, distroHandler DistroHandler,
	snapshotTime imagecustomizerapi.PackageSnapshotTime,
) error {
	ctx, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "configure_packages")
	defer span.End()

	// Delegate the entire package management workflow to the distribution-specific implementation
	return distroHandler.ManagePackages(ctx, buildDir, baseConfigPath, config, imageChroot, toolsChroot,
		rpmsSources, useBaseImageRpmRepos, snapshotTime)
}

func collectPackagesList(baseConfigPath string, packageLists []string, packages []string) ([]string, error) {
	var err error

	// Read in the packages from the package list files.
	var allPackages []string
	for _, packageListRelativePath := range packageLists {
		packageListFilePath := file.GetAbsPathWithBase(baseConfigPath, packageListRelativePath)

		var packageList imagecustomizerapi.PackageList
		err = imagecustomizerapi.UnmarshalAndValidateYamlFile(packageListFilePath, &packageList)
		if err != nil {
			return nil, fmt.Errorf("%w (file='%s'):\n%w", ErrInvalidPackageListFile, packageListFilePath, err)
		}

		allPackages = append(allPackages, packageList.Packages...)
	}

	allPackages = append(allPackages, packages...)
	return allPackages, nil
}

// startInstallPackagesSpan creates a telemetry span for a package install operation with standardized attributes.
// The caller must call span.End() (typically via defer) when the operation completes.
func startInstallPackagesSpan(ctx context.Context, packages []string) (context.Context, trace.Span) {
	ctx, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "install_packages")
	span.SetAttributes(
		attribute.Int("install_packages_count", len(packages)),
		attribute.StringSlice("packages", packages),
	)
	return ctx, span
}

// startUpdatePackagesSpan creates a telemetry span for a package update operation with standardized attributes.
// The caller must call span.End() (typically via defer) when the operation completes.
func startUpdatePackagesSpan(ctx context.Context, packages []string) (context.Context, trace.Span) {
	ctx, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "update_packages")
	span.SetAttributes(
		attribute.Int("update_packages_count", len(packages)),
		attribute.StringSlice("packages", packages),
	)
	return ctx, span
}

// startUpdateExistingPackagesSpan creates a telemetry span for an update of all existing packages with standardized
// attributes. The caller must call span.End() (typically via defer) when the operation completes.
func startUpdateExistingPackagesSpan(ctx context.Context) (context.Context, trace.Span) {
	return otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "update_existing_packages")
}

// startRemovePackagesSpan creates a telemetry span for a package remove operation with standardized attributes.
// The caller must call span.End() (typically via defer) when the operation completes.
func startRemovePackagesSpan(ctx context.Context, packages []string) (context.Context, trace.Span) {
	ctx, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "remove_packages")
	span.SetAttributes(
		attribute.Int("remove_packages_count", len(packages)),
		attribute.StringSlice("packages", packages),
	)
	return ctx, span
}

// startRefreshPackageMetadataSpan creates a telemetry span for a package metadata refresh operation.
// The caller must call span.End() (typically via defer) when the operation completes.
func startRefreshPackageMetadataSpan(ctx context.Context) (context.Context, trace.Span) {
	return otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "refresh_package_metadata")
}

// startCleanPackagesCacheSpan creates a telemetry span for a package cache clean operation.
// The caller must call span.End() (typically via defer) when the operation completes.
func startCleanPackagesCacheSpan(ctx context.Context) (context.Context, trace.Span) {
	return otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "clean_package_cache")
}

func needPackageSources(config *imagecustomizerapi.OS) bool {
	return len(config.Packages.Install) > 0 || len(config.Packages.Update) > 0 || config.Packages.UpdateExistingPackages
}

func needPackageCleanup(config *imagecustomizerapi.OS) bool {
	return needPackageSources(config) || len(config.Packages.Remove) > 0
}

func removeOsPackageManager(ctx context.Context, distroHandler DistroHandler, imageChroot *safechroot.Chroot,
	toolsChroot *safechroot.Chroot, imageUuidStr string,
) error {
	ctx, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "remove_package_manager")
	defer span.End()

	logger.Log.Infof("Removing package manager")

	osManifestPackages, err := distroHandler.RemovePackageManagerTools(ctx, imageChroot, toolsChroot)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrRemovePackageManagerPackages, err)
	}

	doc := createOsManifest(distroHandler, imageUuidStr, osManifestPackages)

	err = writePackageManifest(doc, imageChroot)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrWritePackageManifest, err)
	}

	err = distroHandler.RemovePackageManagerFiles(ctx, imageChroot)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrRemovePackageManagerFilesAndDirs, err)
	}

	return nil
}

func removePackageManagementFiles(imageChroot *safechroot.Chroot, filesAndDirsToRemove []string) error {
	for _, fileToRemove := range filesAndDirsToRemove {
		logger.Log.Debugf("Removing package management file/dir (%s)", fileToRemove)

		err := os.RemoveAll(filepath.Join(imageChroot.RootDir(), fileToRemove))
		if err != nil {
			return fmt.Errorf("failed to remove package management file/dir (%s):\n%w", fileToRemove, err)
		}
	}

	return nil
}

func createOsManifest(distroHandler DistroHandler, imageUuidStr string, osManifestPackages osManifestPackages,
) spdx.Document {
	docId := spdx.ElementID("DOCUMENT")

	targetOs := distroHandler.GetTargetOs()
	imageId := spdx.ElementID(fmt.Sprintf("Package-%s-%s", targetOs.Distro, imageUuidStr))
	imageName := fmt.Sprintf("%s-%s", targetOs.Distro, imageUuidStr)

	doc := spdx.Document{
		SPDXVersion:       spdx.Version,
		DataLicense:       spdx.DataLicense,
		SPDXIdentifier:    docId,
		DocumentName:      imageName,
		DocumentNamespace: fmt.Sprintf("https://spdx.microsoft.com/imagecustomizer/%s", imageUuidStr),
		Packages:          osManifestPackages.Packages,
		Relationships:     osManifestPackages.Relationships,
	}

	// Add relationships from the image "package" to all the system packages.
	for _, packageInfo := range doc.Packages {
		doc.Relationships = append(doc.Relationships, &spdx.Relationship{
			RefA:         spdx.DocElementID{ElementRefID: imageId},
			RefB:         spdx.DocElementID{ElementRefID: packageInfo.PackageSPDXIdentifier},
			Relationship: "CONTAINS",
		})
	}

	// Add relationship from document to the image "package".
	doc.Relationships = append(doc.Relationships, &spdx.Relationship{
		RefA:         spdx.DocElementID{ElementRefID: docId},
		RefB:         spdx.DocElementID{ElementRefID: imageId},
		Relationship: "DESCRIBES",
	})

	// Add the image "package".
	doc.Packages = append(doc.Packages, &spdx.Package{
		PackageName:             imageName,
		PackageSPDXIdentifier:   imageId,
		PackageDownloadLocation: "NOASSERTION",
		FilesAnalyzed:           false,
		PackageLicenseConcluded: "NOASSERTION",
		PackageLicenseDeclared:  "NOASSERTION",
		PackageCopyrightText:    "NOASSERTION",
		PackageSupplier: &spdx.Supplier{
			Supplier: "NOASSERTION",
		},
	})

	return doc
}
