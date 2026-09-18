// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/envfile"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/spdxmanifest"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

const packageManifestPath = "/usr/share/os-manifests/package-manifest.spdx.json"

var (
	ErrPackageManifestRead                = NewImageCustomizerError("PackageManifest:Read", "failed to read the package manifest")
	ErrPackageManifestModeRequired        = NewImageCustomizerError("PackageManifest:ModeRequired", "'os.packages.manifest.mode' must be specified because the base image carries a package manifest")
	ErrPackageManifestCreateRequired      = NewImageCustomizerError("PackageManifest:CreateRequired", "'os.packages.manifest.mode' must be 'create' when exporting a package manifest that is absent from the base image")
	ErrPackageManifestUnknownMode         = NewImageCustomizerError("PackageManifest:UnknownMode", "unknown package manifest mode")
	ErrPackageManifestDelete              = NewImageCustomizerError("PackageManifest:Delete", "failed to delete the package manifest")
	ErrPackageManifestList                = NewImageCustomizerError("PackageManifest:List", "failed to list the image's installed packages")
	ErrPackageManifestNoInstalledPackages = NewImageCustomizerError("PackageManifest:NoInstalledPackages", "the image reported no installed packages")
	ErrPackageManifestVersion             = NewImageCustomizerError("PackageManifest:Version", "failed to read the image version for the package manifest")
	ErrPackageManifestBuild               = NewImageCustomizerError("PackageManifest:Build", "failed to build the package manifest")
	ErrPackageManifestCreateDirectory     = NewImageCustomizerError("PackageManifest:CreateDirectory", "failed to create the package manifest directory")
	ErrPackageManifestWrite               = NewImageCustomizerError("PackageManifest:Write", "failed to write the package manifest")
	ErrPackageManifestOutputMissing       = NewImageCustomizerError("PackageManifest:OutputMissing", "no package manifest to write output path")
	ErrPackageManifestOutputWrite         = NewImageCustomizerError("PackageManifest:OutputWrite", "failed to write the package manifest to output path")
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
			return fmt.Errorf("%w (path='%s')", ErrPackageManifestModeRequired, packageManifestPath)
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
		return fmt.Errorf("%w (path='%s')", ErrPackageManifestCreateRequired, packageManifestPath)
	}

	return nil
}

func applyPackageManifestMode(ctx context.Context, distroHandler DistroHandler, imageChroot *safechroot.Chroot,
	packageManifestMode imagecustomizerapi.PackageManifestMode, buildTime string,
) error {
	if packageManifestMode == imagecustomizerapi.PackageManifestModeUnspecified {
		return nil
	}

	manifestPath := filepath.Join(imageChroot.RootDir(), packageManifestPath)

	ctx, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "apply_package_manifest_mode")
	defer span.End()
	span.SetAttributes(attribute.String("mode", string(packageManifestMode)))

	switch packageManifestMode {
	case imagecustomizerapi.PackageManifestModePassthrough:
		logger.Log.Info("Skipping package manifest changes")

	case imagecustomizerapi.PackageManifestModeNone:
		exists, err := file.PathExists(manifestPath)
		if err != nil {
			return fmt.Errorf("%w (path='%s'):\n%w", ErrPackageManifestRead, manifestPath, err)
		}

		if exists {
			logger.Log.Info("Deleting package manifest")
			err = os.Remove(manifestPath)
			if err != nil {
				return fmt.Errorf("%w (path='%s'):\n%w", ErrPackageManifestDelete, manifestPath, err)
			}
		} else {
			logger.Log.Info("Package manifest does not exist, nothing to delete")
		}

	case imagecustomizerapi.PackageManifestModeCreate:
		logger.Log.Info("Writing package manifest")
		packages, err := distroHandler.ListInstalledPackagesForSpdx(imageChroot)
		if err != nil {
			return fmt.Errorf("%w:\n%w", ErrPackageManifestList, err)
		}
		if len(packages) == 0 {
			return ErrPackageManifestNoInstalledPackages
		}
		err = createPackageManifest(ctx, distroHandler, imageChroot, buildTime, manifestPath, packages)
		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("%w (mode='%s')", ErrPackageManifestUnknownMode, packageManifestMode)
	}

	return nil
}

func createPackageManifest(ctx context.Context, distroHandler DistroHandler, imageChroot safechroot.ChrootInterface,
	buildTime string, manifestPath string, packages []spdxmanifest.Package,
) error {
	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "create_package_manifest")
	defer span.End()

	span.SetAttributes(attribute.Int("package_count", len(packages)))

	versionInfo, err := readPackageManifestVersion(imageChroot)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrPackageManifestVersion, err)
	}
	metadata := spdxmanifest.BuildMetadata{
		Name:        string(distroHandler.GetTargetOs().Distro),
		VersionInfo: versionInfo,
		Created:     buildTime,
	}
	metadata.ToolVersion = ToolVersion
	if metadata.ToolVersion == "" {
		metadata.ToolVersion = "dev"
	}

	manifest, err := spdxmanifest.Build(metadata, packages)
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

func readPackageManifestVersion(imageChroot safechroot.ChrootInterface) (string, error) {
	for _, candidate := range targetos.OsReleaseFileCandidates {
		fields, err := envfile.ParseEnvFile(filepath.Join(imageChroot.RootDir(), candidate))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}

		versionInfo := fields["VERSION"]
		if versionInfo == "" {
			return "", fmt.Errorf("os-release VERSION is empty (path='%s')", candidate)
		}
		if buildID := fields["BUILD_ID"]; buildID != "" {
			versionInfo += "+" + buildID
		}
		return versionInfo, nil
	}

	return "", fmt.Errorf("no os-release file found:\n%w", os.ErrNotExist)
}

func outputPackageManifest(ctx context.Context, imageChroot safechroot.ChrootInterface, outputPath string) error {
	logger.Log.Infof("Extracting package manifest from image")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "output_package_manifest")
	defer span.End()

	manifestPath := filepath.Join(imageChroot.RootDir(), packageManifestPath)

	exists, err := file.PathExists(manifestPath)
	if err != nil {
		return fmt.Errorf("%w (path='%s'):\n%w", ErrPackageManifestRead, manifestPath, err)
	}

	if !exists {
		return fmt.Errorf("%w (path='%s')", ErrPackageManifestOutputMissing, packageManifestPath)
	}

	err = file.Copy(manifestPath, outputPath)
	if err != nil {
		return fmt.Errorf("%w (path='%s'):\n%w", ErrPackageManifestOutputWrite, outputPath, err)
	}

	logger.Log.Infof("Successfully extracted package manifest to %s", outputPath)

	return nil
}
