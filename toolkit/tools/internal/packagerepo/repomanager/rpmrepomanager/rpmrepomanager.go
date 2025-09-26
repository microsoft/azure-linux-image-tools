// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package rpmrepomanager

import (
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/packagerepo/repoutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
)

// CreateOrUpdateRepo will create an RPM repository at repoDir or update
// it if the metadata files already exist.
func CreateOrUpdateRepo(repoDir string) (err error) {
	// Check if createrepo command is available
	createRepoCmd, err := repoutils.FindCreateRepoCommand()
	if err != nil {
		return fmt.Errorf("unable to create repo:\n%w", err)
	}

	// Create or update repodata
	_, stderr, err := shell.Execute(createRepoCmd, "--compatibility", "--update", repoDir)
	if err != nil {
		logger.Log.Warn(stderr)
	}

	return
}
