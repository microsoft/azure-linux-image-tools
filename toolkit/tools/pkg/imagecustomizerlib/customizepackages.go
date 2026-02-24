// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var (
	// Package-related errors
	ErrPackageRepoMetadataRefresh = NewImageCustomizerError("Packages:RepoMetadataRefresh", "failed to refresh repo metadata")
	ErrInvalidPackageListFile     = NewImageCustomizerError("Packages:InvalidPackageListFile", "failed to read package list file")
	ErrPackageRemove              = NewImageCustomizerError("Packages:Remove", "failed to remove packages")
	ErrPackageUpdate              = NewImageCustomizerError("Packages:Update", "failed to update packages")
	ErrPackagesUpdateInstalled    = NewImageCustomizerError("Packages:UpdateInstalled", "failed to update installed packages")
	ErrPackageInstall             = NewImageCustomizerError("Packages:Install", "failed to install packages")
	ErrPackageCacheClean          = NewImageCustomizerError("Packages:CacheClean", "failed to clean cache")
	ErrMountRpmSources            = NewImageCustomizerError("Packages:MountRpmSources", "failed to mount RPM sources")
	ErrSnapshotTimeNotSupported   = NewImageCustomizerError("Packages:SnapshotTimeNotSupported", "snapshot time is not supported")
)

const (
	// Shared telemetry span names for package management operations.
	packageSpanNameRefreshMetadata = "refresh_metadata"
	packageSpanNameCleanCache      = "clean_cache"
	packageActionInstall           = "install"
	packageActionUpdate            = "update"
	packageActionRemove            = "remove"
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

// startPackagesSpan creates a telemetry span for a package management operation.
// The caller must call span.End() (typically via defer) when the operation completes.
func startPackagesSpan(ctx context.Context, spanName string) (context.Context, trace.Span) {
	return otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, spanName)
}

// startPackageListSpan creates a telemetry span for a package operation (e.g. "install", "update")
// with standardized attributes for the package count and list.
// The caller must call span.End() (typically via defer) when the operation completes.
func startPackageListSpan(ctx context.Context, action string, packages []string) (context.Context, trace.Span) {
	spanName := fmt.Sprint(action, "_packages")
	ctx, span := startPackagesSpan(ctx, spanName)
	span.SetAttributes(
		attribute.Int(fmt.Sprint(spanName, "_count"), len(packages)),
		attribute.StringSlice("packages", packages),
	)
	return ctx, span
}

func isPackageInstalled(imageChroot safechroot.ChrootInterface, packageName string) bool {
	err := imageChroot.UnsafeRun(func() error {
		_, _, err := shell.Execute("tdnf", "info", packageName, "--repo", "@system")
		return err
	})
	if err != nil {
		return false
	}
	return true
}
