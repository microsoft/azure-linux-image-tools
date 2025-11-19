// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/configuration"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/installutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sys/unix"
)

var (
	// SELinux-related errors
	ErrSELinuxGetCurrentMode  = NewImageCustomizerError("SELinux:GetCurrentMode", "failed to get current SELinux mode")
	ErrSELinuxConfigFileCheck = NewImageCustomizerError("SELinux:ConfigFileCheck", "failed to check if SELinux config file exists")
	ErrSELinuxPolicyMissing   = NewImageCustomizerError("SELinux:PolicyMissing", "SELinux is enabled but policy file is missing")
	ErrSELinuxConfigUpdate    = NewImageCustomizerError("SELinux:ConfigUpdate", "failed to set SELinux mode in config file")
	ErrSELinuxRelabelFiles    = NewImageCustomizerError("SELinux:RelabelFiles", "failed to set SELinux file labels")
)

func handleSELinux(ctx context.Context, buildDir string, selinuxMode imagecustomizerapi.SELinuxMode, resetBootLoaderType imagecustomizerapi.ResetBootLoaderType,
	imageChroot *safechroot.Chroot,
) (imagecustomizerapi.SELinuxMode, error) {
	var err error

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "configure_selinux")
	span.SetAttributes(
		attribute.String("selinux_mode", string(selinuxMode)),
	)
	defer span.End()

	bootCustomizer, err := NewBootCustomizer(imageChroot)
	if err != nil {
		return imagecustomizerapi.SELinuxModeDefault, err
	}

	if selinuxMode == imagecustomizerapi.SELinuxModeDefault {
		// No changes to the SELinux have been requested.
		// So, return the current SELinux mode.
		currentSELinuxMode, err := bootCustomizer.GetSELinuxMode(buildDir, imageChroot)
		if err != nil {
			return imagecustomizerapi.SELinuxModeDefault, fmt.Errorf("%w:\n%w", ErrSELinuxGetCurrentMode, err)
		}

		return currentSELinuxMode, nil
	}

	logger.Log.Infof("Configuring SELinux mode")

	switch resetBootLoaderType {
	case imagecustomizerapi.ResetBootLoaderTypeHard:
		// The grub.cfg file has been recreated from scratch and therefore the SELinux args will already be correct and
		// don't need to be updated.

	default:
		// Update the SELinux kernel command-line args.
		err := bootCustomizer.UpdateSELinuxCommandLine(selinuxMode)
		if err != nil {
			return imagecustomizerapi.SELinuxModeDefault, err
		}

		err = bootCustomizer.WriteToFile(imageChroot)
		if err != nil {
			return imagecustomizerapi.SELinuxModeDefault, err
		}
	}

	err = UpdateSELinuxModeInConfigFile(selinuxMode, imageChroot)
	if err != nil {
		return imagecustomizerapi.SELinuxModeDefault, err
	}

	return selinuxMode, nil
}

func UpdateSELinuxModeInConfigFile(selinuxMode imagecustomizerapi.SELinuxMode, imageChroot safechroot.ChrootInterface) error {
	imagerSELinuxMode, err := selinuxModeToImager(selinuxMode)
	if err != nil {
		return err
	}

	selinuxConfigFileFullPath := filepath.Join(imageChroot.RootDir(), installutils.SELinuxConfigFile)
	selinuxConfigFileExists, err := file.PathExists(selinuxConfigFileFullPath)
	if err != nil {
		return fmt.Errorf("%w (file='%s'):\n%w", ErrSELinuxConfigFileCheck, installutils.SELinuxConfigFile, err)
	}

	// Ensure an SELinux policy has been installed.
	// Typically, this is provided by the 'selinux-policy' package.
	if selinuxMode != imagecustomizerapi.SELinuxModeDisabled && !selinuxConfigFileExists {
		return fmt.Errorf("%w (file='%s'):\n"+
			"please ensure an SELinux policy is installed:\n"+
			"the '%s' package provides the default policy",
			ErrSELinuxPolicyMissing, installutils.SELinuxConfigFile, configuration.SELinuxPolicyDefault)
	}

	if selinuxConfigFileExists {
		err = installutils.SELinuxUpdateConfig(imagerSELinuxMode, imageChroot)
		if err != nil {
			return fmt.Errorf("%w:\n%w", ErrSELinuxConfigUpdate, err)
		}
	}

	return nil
}

func selinuxSetFiles(ctx context.Context, selinuxMode imagecustomizerapi.SELinuxMode, imageChroot *safechroot.Chroot) error {
	if selinuxMode == imagecustomizerapi.SELinuxModeDisabled {
		// SELinux is disabled in the kernel command line.
		// So, no need to call setfiles.
		return nil
	}

	logger.Log.Infof("Setting file SELinux labels")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "set_selinux_files")
	defer span.End()

	// Get the list of mount points.
	mountPointToFsTypeMap := make(map[string]string, 0)
	for _, mountPoint := range getNonSpecialChrootMountPoints(imageChroot) {
		if (mountPoint.GetFlags() & unix.MS_RDONLY) != 0 {
			// Skip read-only filesystems.
			continue
		}

		mountPointToFsTypeMap[mountPoint.GetTarget()] = mountPoint.GetFSType()
	}

	// Set the SELinux config file and relabel all the files.
	err := installutils.SELinuxRelabelFiles(imageChroot, mountPointToFsTypeMap, false)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrSELinuxRelabelFiles, err)
	}

	return nil
}
