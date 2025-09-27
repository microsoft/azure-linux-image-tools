// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package repoutils

import (
	"fmt"
	"os/exec"
)

// FindCreateRepoCommand searches for createrepo or createrepo_c in $PATH and returns the first one found
func FindCreateRepoCommand() (cmd string, err error) {
	creatrepo_cmds := []string{"createrepo_c", "createrepo"}

	selectedCmd := ""
	// Check if a command exists in the $PATH
	for _, cmd := range creatrepo_cmds {
		_, err = exec.LookPath(cmd)
		if err == nil {
			selectedCmd = cmd
			break
		}
	}

	if selectedCmd == "" {
		return "", fmt.Errorf("failed to find a working createrepo command.\nattempted commands: %v", creatrepo_cmds)
	}

	return selectedCmd, nil
}
