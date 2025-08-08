// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package vhdutils

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
)

var (
	testsTempDir string
)

func TestMain(m *testing.M) {
	var err error

	logger.InitStderrLog()

	flag.Parse()

	workingDir, err := os.Getwd()
	if err != nil {
		logger.Log.Panicf("Failed to get working directory, error: %s", err)
	}

	testsTempDir = filepath.Join(workingDir, "_tmp")

	err = os.MkdirAll(testsTempDir, os.ModePerm)
	if err != nil {
		logger.Log.Panicf("Failed to create test temp directory, error: %s", err)
	}

	retVal := m.Run()

	err = os.RemoveAll(testsTempDir)
	if err != nil {
		logger.Log.Warnf("Failed to cleanup test temp dir (%s). Error: %s", testsTempDir, err)
	}

	os.Exit(retVal)
}
