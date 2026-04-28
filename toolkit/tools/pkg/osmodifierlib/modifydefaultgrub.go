// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package osmodifierlib

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/pkg/imagecustomizerlib"
)

var grubArgs = []string{
	"rd.overlayfs",
	"roothash",
	"root",
	"selinux",
	"enforcing",
}

func modifyDefaultGrub() error {
	var dummyChroot safechroot.ChrootInterface = &safechroot.DummyChroot{}

	buildDir, err := os.MkdirTemp("", "osmodifier-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary build directory: %w", err)
	}
	defer os.RemoveAll(buildDir)

	distroHandler, err := imagecustomizerlib.NewDistroHandlerFromChroot(dummyChroot)
	if err != nil {
		return fmt.Errorf("failed to detect distribution:\n%w", err)
	}

	// Get verity, selinux, overlayfs, and root device values from /boot/grub2/grub.cfg
	values, rootDevice, err := extractValuesFromGrubConfig(dummyChroot, distroHandler)
	if err != nil {
		return fmt.Errorf("error getting verity, selinux and overlayfs values from grub.cfg:\n%w", err)
	}

	bootCustomizer, err := imagecustomizerlib.NewBootCustomizer(dummyChroot, nil, buildDir, distroHandler)
	if err != nil {
		return err
	}

	// Stamp verity, selinux and overlayfs values to /etc/default/grub
	err = bootCustomizer.UpdateKernelCommandLineArgs("GRUB_CMDLINE_LINUX", grubArgs, values)
	if err != nil {
		return err
	}

	// Stamp root device to /etc/default/grub
	err = bootCustomizer.SetRootDevice(rootDevice)
	if err != nil {
		return err
	}

	err = bootCustomizer.WriteToFile(dummyChroot)
	if err != nil {
		return fmt.Errorf("error writing to default grub:\n%w", err)
	} else {
		logger.Log.Info("Successfully updated default grub")
	}

	return nil
}

func extractValuesFromGrubConfig(imageChroot safechroot.ChrootInterface, distroHandler imagecustomizerlib.DistroHandler,
) ([]string, string, error) {
	bootDir := filepath.Join(imageChroot.RootDir(), "boot")
	argMap, err := distroHandler.ReadNonRecoveryKernelCmdlines(bootDir, grubArgs)
	if err != nil {
		return nil, "", err
	}

	var values []string
	rootDevice := argMap["root"]
	delete(argMap, "root")
	for name, value := range argMap {
		values = append(values, name+"="+value)
	}

	return values, rootDevice, nil
}
