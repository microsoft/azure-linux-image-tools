// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package systemd

import (
	"fmt"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/sirupsen/logrus"
)

// IsServiceEnabled checks if a service is enabled or disabled.
func IsServiceEnabled(name string, imageChroot safechroot.ChrootInterface) (bool, error) {
	serviceEnabled := true
	stdout, _, err := shell.NewExecBuilder("systemctl", "is-enabled", name).
		LogLevel(logrus.DebugLevel, logrus.DebugLevel).
		ErrorStderrLines(1).
		Chroot(imageChroot.ChrootDir()).
		ExecuteCaptureOutput()

	// `systemctl is-enabled` returns:
	//   enabled:  Exit code = 0, stdout = "enabled"
	//   disabled: Exit code = 1, stdout = "disabled"
	//   error:    Exit code = 1, stdout = ""
	if err != nil {
		if strings.TrimSpace(stdout) != "disabled" {
			return false, fmt.Errorf("failed to check if (%s) service is enabled:\n%w", name, err)
		}

		serviceEnabled = false
	}

	return serviceEnabled, nil
}
