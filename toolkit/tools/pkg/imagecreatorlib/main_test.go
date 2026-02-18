package imagecreatorlib

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
)

var (
	testDir      string
	tmpDir       string
	workingDir   string
	testutilsDir string
)

var runCreateImageTests = flag.Bool("run-create-image-tests", false, "Runs the tests that create images")

func TestMain(m *testing.M) {
	var err error

	logger.InitStderrLog()

	flag.Parse()

	workingDir, err = os.Getwd()
	if err != nil {
		logger.Log.Panicf("Failed to get working directory, error: %s", err)
	}

	testDir = filepath.Join(workingDir, "../imagecustomizerlib/testdata")
	tmpDir = filepath.Join(workingDir, "_tmp")

	testutilsDir = filepath.Join(workingDir, "../../internal/testutils")

	err = os.MkdirAll(tmpDir, os.ModePerm)
	if err != nil {
		logger.Log.Panicf("Failed to create tmp directory, error: %s", err)
	}

	retVal := m.Run()

	err = os.RemoveAll(tmpDir)
	if err != nil {
		logger.Log.Warnf("Failed to cleanup tmp dir (%s). Error: %s", tmpDir, err)
	}

	os.Exit(retVal)
}

// Skip the test if requirements for testing CreateImage() are not met.
func checkSkipForCreateImage(t *testing.T, runCreateImageTests *bool) {
	testutils.CheckSkipForCustomizeImageRequirements(t)

	if !*runCreateImageTests {
		t.Skipf("Skipping test because --run-create-image-tests is not set")
	}
}
