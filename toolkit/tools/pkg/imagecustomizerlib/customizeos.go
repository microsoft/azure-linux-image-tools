// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"time"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
)

const (
	buildTimeFormat = "2006-01-02T15:04:05Z"
)

func doOsCustomizations(ctx context.Context, rc *ResolvedConfig, imageConnection *imageconnection.ImageConnection,
	partitionsCustomized bool, partUuidToFstabEntry map[string]diskutils.FstabEntry, distroHandler distroHandler,
) error {
	var err error

	imageChroot := imageConnection.Chroot()

	buildTime := time.Now().Format(buildTimeFormat)

	resolvConf, err := overrideResolvConf(imageChroot)
	if err != nil {
		return err
	}

	err = UpdateHostname(ctx, rc.Config.OS.Hostname, imageChroot)
	if err != nil {
		return err
	}

	for _, configWithBase := range rc.ConfigChain {
		err = addRemoveAndUpdatePackages(ctx, rc.BuildDirAbs, rc.BaseConfigPath, configWithBase.Config.OS,
			imageChroot, nil, rc.Options.RpmsSources, rc.Options.UseBaseImageRpmRepos, distroHandler,
			configWithBase.Config.OS.Packages.SnapshotTime)
		if err != nil {
			return err
		}

		err = AddOrUpdateGroups(ctx, configWithBase.Config.OS.Groups, imageChroot)
		if err != nil {
			return err
		}

		err = AddOrUpdateUsers(ctx, configWithBase.Config.OS.Users, configWithBase.BaseConfigPath, imageChroot)
		if err != nil {
			return err
		}

		err = copyAdditionalDirs(ctx, rc.BaseConfigPath, configWithBase.Config.OS.AdditionalDirs, imageChroot)
		if err != nil {
			return err
		}

		err = copyAdditionalFiles(ctx, rc.BaseConfigPath, configWithBase.Config.OS.AdditionalFiles, imageChroot)
		if err != nil {
			return err
		}

		err = EnableOrDisableServices(ctx, configWithBase.Config.OS.Services, imageChroot)
		if err != nil {
			return err
		}

		err = LoadOrDisableModules(ctx, configWithBase.Config.OS.Modules, imageChroot.RootDir())
		if err != nil {
			return err
		}

	}

	err = addCustomizerRelease(ctx, imageChroot.RootDir(), ToolVersion, buildTime, rc.ImageUuidStr)
	if err != nil {
		return err
	}

	if rc.Config.OS.ImageHistory != imagecustomizerapi.ImageHistoryNone {
		err = addImageHistory(ctx, imageChroot, rc.ImageUuidStr, rc.BaseConfigPath, ToolVersion, buildTime, rc.Config)
		if err != nil {
			return err
		}
	}

	err = handleBootLoader(ctx, rc.BaseConfigPath, rc.Config, imageConnection, partUuidToFstabEntry, false)
	if err != nil {
		return err
	}

	selinuxMode, err := handleSELinux(ctx, rc.Config.OS.SELinux.Mode, rc.Config.OS.BootLoader.ResetType,
		imageChroot)
	if err != nil {
		return err
	}

	overlayUpdated, err := enableOverlays(ctx, rc.Config.OS.Overlays, selinuxMode, imageChroot)
	if err != nil {
		return err
	}

	verityUpdated, err := enableVerityPartition(ctx, rc.Config.Storage.Verity, imageChroot, distroHandler)
	if err != nil {
		return err
	}

	if partitionsCustomized || overlayUpdated || verityUpdated {
		err = regenerateInitrd(ctx, imageChroot)
		if err != nil {
			return err
		}
	}

	err = runUserScripts(ctx, rc.BaseConfigPath, rc.Config.Scripts.PostCustomization, "postCustomization", imageChroot)
	if err != nil {
		return err
	}

	err = prepareUki(ctx, rc.BuildDirAbs, rc.Config.OS.Uki, imageChroot, distroHandler)
	if err != nil {
		return err
	}

	err = restoreResolvConf(ctx, resolvConf, imageChroot)
	if err != nil {
		return err
	}

	err = selinuxSetFiles(ctx, selinuxMode, imageChroot)
	if err != nil {
		return err
	}

	err = runUserScripts(ctx, rc.BaseConfigPath, rc.Config.Scripts.FinalizeCustomization, "finalizeCustomization",
		imageChroot)
	if err != nil {
		return err
	}

	err = checkForInstalledKernel(ctx, imageChroot)
	if err != nil {
		return err
	}

	return nil
}
