// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package toolschroot

import (
	"os"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
)

func TestMain(m *testing.M) {
	logger.InitStderrLog()
	os.Exit(m.Run())
}
