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
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/spdxmanifest"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

const packageManifestPath = "usr/share/os-manifests/package-manifest.spdx.json"

var (
	ErrPackageManifestRead                = NewImageCustomizerError("Packages:PackageManifestRead", "failed to read the package manifest")
	ErrPackageManifestModeRequired        = NewImageCustomizerError("Packages:PackageManifestModeRequired", "'os.packages.manifest.mode' must be specified because the base image carries a package manifest")
	ErrPackageManifestCreateRequired      = NewImageCustomizerError("Packages:PackageManifestCreateRequired", "'os.packages.manifest.mode' must be 'create' when exporting a package manifest that is absent from the base image")
	ErrPackageManifestUnknownMode         = NewImageCustomizerError("Packages:PackageManifestUnknownMode", "unknown package manifest mode")
	ErrPackageManifestDelete              = NewImageCustomizerError("Packages:PackageManifestDelete", "failed to delete the package manifest")
	ErrPackageManifestList                = NewImageCustomizerError("Packages:PackageManifestList", "failed to list the image's installed packages")
	ErrPackageManifestNoInstalledPackages = NewImageCustomizerError("Packages:PackageManifestNoInstalledPackages", "the image reported no installed packages")
	ErrPackageManifestDuplicatePackage    = NewImageCustomizerError("Packages:PackageManifestDuplicatePackage", "the image reported a duplicate installed package ID")
	ErrPackageManifestRemovalNotInstalled = NewImageCustomizerError("Packages:PackageManifestRemovalNotInstalled", "removal transaction contains a package that is not installed")
	ErrPackageManifestVersion             = NewImageCustomizerError("Packages:PackageManifestVersion", "failed to read the image version for the package manifest")
	ErrPackageManifestBuild               = NewImageCustomizerError("Packages:PackageManifestBuild", "failed to build the package manifest")
	ErrPackageManifestCreateDirectory     = NewImageCustomizerError("Packages:PackageManifestCreateDirectory", "failed to create the package manifest directory")
	ErrPackageManifestWrite               = NewImageCustomizerError("Packages:PackageManifestWrite", "failed to write the package manifest")
	ErrPackageManifestOutputMissing       = NewImageCustomizerError("Packages:PackageManifestOutputMissing", "no package manifest to write output path")
	ErrPackageManifestOutputWrite         = NewImageCustomizerError("Packages:PackageManifestOutputWrite", "failed to write the package manifest to output path")
	ErrPackageManifestImageConnection     = NewImageCustomizerError("Packages:PackageManifestImageConnection", "failed to connect to the image to read the package manifest")
)

func validateBaseImagePackageManifest(rc *ResolvedConfig, rootDir string) error {
	// Any explicit mode is allowed when no outfile file is requested, regardless of base image state.
	if rc.PackageManifestMode != imagecustomizerapi.PackageManifestModeUnspecified && rc.OutputPackageManifestPath == "" {
		return nil
	}

	manifestPath := filepath.Join(rootDir, packageManifestPath)
	exists, err := file.PathExists(manifestPath)
	if err != nil {
		return fmt.Errorf("%w (path='%s'):\n%w", ErrPackageManifestRead, manifestPath, err)
	}

	if exists {
		// Require an explicit mode when there is an existing manifest so it isn't unintentionally left stale.
		if rc.PackageManifestMode == imagecustomizerapi.PackageManifestModeUnspecified {
			return fmt.Errorf("%w (path='/%s')", ErrPackageManifestModeRequired, packageManifestPath)
		}
		return nil
	}

	// Mode isn't necessary when there is no existing manifest to get stale and no output file that needs to be produced.
	if rc.OutputPackageManifestPath == "" {
		return nil
	}

	// The output file requires creating a manifest when the base image has none to copy.
	// Other modes do not create the manifest.
	if rc.PackageManifestMode != imagecustomizerapi.PackageManifestModeCreate {
		return fmt.Errorf("%w (path='/%s')", ErrPackageManifestCreateRequired, packageManifestPath)
	}

	return nil
}

// preparePackageManifestCreate captures the installed packages before package manager removal.
// It requires a non-empty package list and an image version for manifest creation.
func preparePackageManifestCreate(ctx context.Context, distroHandler DistroHandler, imageChroot *safechroot.Chroot,
	toolsChroot *safechroot.Chroot, packageManifestMode imagecustomizerapi.PackageManifestMode,
) ([]spdxmanifest.Package, error) {
	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "prepare_package_manifest")
	defer span.End()

	packages, err := distroHandler.ListInstalledPackages(imageChroot, toolsChroot)
	if err != nil {
		return nil, fmt.Errorf("%w:\n%w", ErrPackageManifestList, err)
	}
	if len(packages) == 0 {
		return nil, ErrPackageManifestNoInstalledPackages
	}

	return packages, nil
}

// applyPackageManifestMode applies the specified mode using the snapshot taken before package manager removal.
// installedPackages and removedPackages are only used in "create" mode.
func applyPackageManifestMode(ctx context.Context, distroHandler DistroHandler, imageChroot *safechroot.Chroot,
	toolsChroot *safechroot.Chroot, packageManifestMode imagecustomizerapi.PackageManifestMode,
	buildTime string, installedPackages []spdxmanifest.Package, removedPackages []string,
) error {
	if packageManifestMode == imagecustomizerapi.PackageManifestModeUnspecified {
		return nil
	}

	manifestPath := filepath.Join(imageChroot.RootDir(), packageManifestPath)
	exists, err := file.PathExists(manifestPath)
	if err != nil {
		return fmt.Errorf("%w (path='%s'):\n%w", ErrPackageManifestRead, manifestPath, err)
	}

	ctx, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "apply_package_manifest_mode")
	defer span.End()
	span.SetAttributes(attribute.String("mode", string(packageManifestMode)))

	switch packageManifestMode {
	case imagecustomizerapi.PackageManifestModePassthrough:
		logger.Log.Infof("Skipping package manifest changes (mode='%s')", packageManifestMode)

	case imagecustomizerapi.PackageManifestModeNone:
		if exists {
			logger.Log.Infof("Deleting package manifest (mode='%s')", packageManifestMode)
			err = os.Remove(manifestPath)
			if err != nil {
				return fmt.Errorf("%w (path='%s'):\n%w", ErrPackageManifestDelete, manifestPath, err)
			}
		}

	case imagecustomizerapi.PackageManifestModeCreate:
		logger.Log.Infof("Writing package manifest (mode='%s')", packageManifestMode)
		packages, err := subtractPackages(installedPackages, removedPackages)
		if err != nil {
			return err
		}
		err = createPackageManifest(ctx, distroHandler, buildTime, manifestPath, packages)
		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("%w (mode='%s')", ErrPackageManifestUnknownMode, packageManifestMode)
	}

	return nil
}

func createPackageManifest(ctx context.Context, distroHandler DistroHandler, buildTime string, manifestPath string,
	packages []spdxmanifest.Package,
) error {
	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "create_package_manifest")
	defer span.End()

	span.SetAttributes(attribute.Int("package_count", len(packages)))

	options := distroHandler.GetPackageManifestBuildOptions(buildTime)
	if options.VersionInfo == "" {
		return ErrPackageManifestVersion
	}
	options.ToolVersion = ToolVersion
	if options.ToolVersion == "" {
		options.ToolVersion = "dev"
	}

	manifest, err := spdxmanifest.Build(options, packages)
	if err != nil {
		return fmt.Errorf("%w (path='%s'):\n%w", ErrPackageManifestBuild, manifestPath, err)
	}

	err = os.MkdirAll(filepath.Dir(manifestPath), 0o755)
	if err != nil {
		return fmt.Errorf("%w (path='%s'):\n%w", ErrPackageManifestCreateDirectory, manifestPath, err)
	}

	err = file.WriteWithPerm(string(manifest), manifestPath, 0o644)
	if err != nil {
		return fmt.Errorf("%w (path='%s'):\n%w", ErrPackageManifestWrite, manifestPath, err)
	}

	return nil
}

func subtractPackages(installed []spdxmanifest.Package, removed []string) ([]spdxmanifest.Package, error) {
	installedIds := make(map[string]struct{}, len(installed))
	for _, pkg := range installed {
		if _, found := installedIds[pkg.ID]; found {
			return nil, fmt.Errorf("%w (id='%s')", ErrPackageManifestDuplicatePackage, pkg.ID)
		}
		installedIds[pkg.ID] = struct{}{}
	}

	removedIDs := make(map[string]struct{}, len(removed))
	for _, pkgId := range removed {
		if _, found := installedIds[pkgId]; !found {
			return nil, fmt.Errorf("%w (id='%s')", ErrPackageManifestRemovalNotInstalled, pkgId)
		}
		removedIDs[pkgId] = struct{}{}
	}

	remainingPackages := make([]spdxmanifest.Package, 0, len(installed))
	for _, packageInfo := range installed {
		if _, found := removedIDs[packageInfo.ID]; !found {
			remainingPackages = append(remainingPackages, packageInfo)
		}
	}

	return remainingPackages, nil
}

func outputPackageManifest(ctx context.Context, outputPath string, buildDir string, buildImage string,
	partitionsLayout []fstabEntryPartNum, distroHandler DistroHandler,
) error {
	logger.Log.Infof("Extracting package manifest from image")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "output_package_manifest")
	defer span.End()

	imageMountPoint := filepath.Join(buildDir, "package-manifest-extract")

	imageConnection, _, err := reconnectToExistingImage(ctx, buildImage, buildDir, imageMountPoint,
		false /*includeDefaultMounts*/, true /*readonly*/, true /*readOnlyVerity*/, partitionsLayout, distroHandler)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrPackageManifestImageConnection, err)
	}
	defer imageConnection.Close()

	err = copyPackageManifest(outputPath, imageConnection.Chroot().RootDir())
	if err != nil {
		return err
	}

	err = imageConnection.CleanClose()
	if err != nil {
		return fmt.Errorf("failed to cleanly close image connection:\n%w", err)
	}

	logger.Log.Infof("Successfully extracted package manifest to %s", outputPath)

	return nil
}

func copyPackageManifest(outputPath string, imageRootDir string) error {
	manifestPath := filepath.Join(imageRootDir, packageManifestPath)

	exists, err := file.PathExists(manifestPath)
	if err != nil {
		return fmt.Errorf("%w (path='%s'):\n%w", ErrPackageManifestRead, manifestPath, err)
	}

	if !exists {
		return fmt.Errorf("%w (path='/%s')", ErrPackageManifestOutputMissing, packageManifestPath)
	}

	err = file.Copy(manifestPath, outputPath)
	if err != nil {
		return fmt.Errorf("%w (path='%s'):\n%w", ErrPackageManifestOutputWrite, outputPath, err)
	}

	return nil
}
