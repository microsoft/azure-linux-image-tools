// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package network

import (
	"strings"
	"time"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/retry"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
)

// CheckNetworkAccess checks whether there is network access
func CheckNetworkAccess() (err error, hasNetworkAccess bool) {
	const (
		retryAttempts = 10
		retryDuration = time.Second
		squashErrors  = false
		activeStatus  = "active"
	)

	err = retry.Run(func() error {
		err := shell.ExecuteLive(squashErrors, "systemctl", "restart", "systemd-networkd-wait-online")
		if err != nil {
			logger.Log.Errorf("Cannot start systemd-networkd-wait-online.service")
			return err
		}

		stdout, stderr, err := shell.Execute("systemctl", "is-active", "systemd-networkd-wait-online")
		if err != nil {
			logger.Log.Errorf("Failed to query status of systemd-networkd-wait-online: %v", stderr)
			return err
		}

		serviceStatus := strings.TrimSpace(stdout)
		hasNetworkAccess = serviceStatus == activeStatus
		if !hasNetworkAccess {
			logger.Log.Warnf("No network access yet")
		}

		return err
	}, retryAttempts, retryDuration)

	if err != nil {
		logger.Log.Errorf("Failure in multiple attempts to check network access")
	}

	return
}
