// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package osmodifierlib

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/osmodifierapi"
)

func ModifyOSWithConfigFile(ctx context.Context, configFile string) error {
	var err error

	var osConfig osmodifierapi.OS
	err = imagecustomizerapi.UnmarshalAndValidateYamlFile(configFile, &osConfig)
	if err != nil {
		return err
	}

	baseConfigPath, _ := filepath.Split(configFile)

	absBaseConfigPath, err := filepath.Abs(baseConfigPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path of config file directory:\n%w", err)
	}

	err = ModifyOS(ctx, absBaseConfigPath, &osConfig)
	if err != nil {
		return err
	}

	return nil
}

func ModifyOS(ctx context.Context, baseConfigPath string, osConfig *osmodifierapi.OS) error {
	err := doModifications(ctx, baseConfigPath, osConfig)
	if err != nil {
		return err
	}

	return nil
}

func ModifyDefaultGrub() error {
	err := modifyDefaultGrub()
	if err != nil {
		return err
	}

	return nil
}
