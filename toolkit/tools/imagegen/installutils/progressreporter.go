// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package installutils

import (
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
)

// ReportActionf emits the formatted current action being performed on stdout, only if EnableEmittingProgress was invoked with true.
// It also prints the output to the log at debug level regardless of EnableEmittingProgress
func ReportActionf(format string, args ...interface{}) {
	ReportAction(fmt.Sprintf(format, args...))
}

// ReportAction emits the current action being performed on stdout, only if EnableEmittingProgress was invoked with true.
// It also prints the output to the log at debug level regardless of EnableEmittingProgress.
func ReportAction(status string) {
	logger.Log.Debugf("ReportAction: '%s'", status)
}
