// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package diskutils

import (
	"os"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
)

func TestMain(m *testing.M) {
	logger.InitStderrLog()

	retVal := m.Run()
	os.Exit(retVal)
}
