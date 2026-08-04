// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/sirupsen/logrus"
)

// TDNF Package Manager Implementation
type tdnfPackageManager struct {
	version string
}

func newTdnfPackageManager(version string) *tdnfPackageManager {
	return &tdnfPackageManager{version: version}
}

func (pm *tdnfPackageManager) getReleaseVersion() string { return pm.version }

// getCacheOnlyOptions returns TDNF-specific cache options for install/update operations
func (pm *tdnfPackageManager) getCacheOnlyOptions() []string {
	return nil // TDNF doesn't need additional cache options
}

// TDNF-specific constants and output handling
const (
	tdnfTransactionErrorPattern = `^Found \d+ problems$`
	tdnfDownloadPattern         = `^\s*([a-zA-Z0-9\-._+]+)\s+\d+\%\s+\d+$`
)

var (
	tdnfOpLines = []string{
		"Installing/Updating: ",
		"Removing: ",
	}

	tdnfSummaryLines = []string{
		"Installing:",
		"Upgrading:",
		"Removing:",
	}

	tdnfTransactionErrorRegex = regexp.MustCompile(tdnfTransactionErrorPattern)
	tdnfDownloadRegex         = regexp.MustCompile(tdnfDownloadPattern)
)

func (pm *tdnfPackageManager) configureSnapshotTime(packageManagerChroot *safechroot.Chroot,
	snapshotTime imagecustomizerapi.PackageSnapshotTime,
) (func() error, error) {
	cleanup := func() error {
		return cleanupSnapshotTimeConfig(packageManagerChroot)
	}

	// Setup Azure Linux specific TDNF configuration with snapshot
	err := createTempTdnfConfigWithSnapshot(packageManagerChroot, snapshotTime)
	if err != nil {
		return nil, err
	}

	return cleanup, nil
}

func (pm *tdnfPackageManager) executeCommand(args []string, imageChroot *safechroot.Chroot,
	toolsChroot *safechroot.Chroot,
) error {
	pmChroot := imageChroot
	if toolsChroot != nil {
		pmChroot = toolsChroot
	}

	fullArgs := []string{"-v"}

	if _, err := os.Stat(filepath.Join(pmChroot.RootDir(), customTdnfConfRelPath)); err == nil {
		fullArgs = append(fullArgs, "--config", "/"+customTdnfConfRelPath)
	}

	fullArgs = append(fullArgs, args...)

	lastDownloadPackageSeen := ""
	inSummary := false
	seenTransactionErrorMessage := false

	stdoutCallback := func(line string) {
		if !seenTransactionErrorMessage {
			seenTransactionErrorMessage = tdnfTransactionErrorRegex.MatchString(line)
		}

		switch {
		case seenTransactionErrorMessage:
			logger.Log.Warn(line)

		case inSummary && line == "":
			inSummary = false
			logger.Log.Trace(line)

		case inSummary:
			logger.Log.Debug(line)

		case slices.Contains(tdnfSummaryLines, line):
			inSummary = true
			logger.Log.Debug(line)

		case slices.ContainsFunc(tdnfOpLines, func(opPrefix string) bool { return strings.HasPrefix(line, opPrefix) }):
			logger.Log.Debug(line)

		default:
			match := tdnfDownloadRegex.FindStringSubmatch(line)
			if match != nil {
				packageName := match[1]
				if packageName != lastDownloadPackageSeen {
					lastDownloadPackageSeen = packageName
					logger.Log.Debug(line)
					return
				}
			}
			logger.Log.Trace(line)
		}
	}

	return shell.NewExecBuilder(packageManagerTDNF, fullArgs...).
		StdoutCallback(stdoutCallback).
		LogLevel(shell.LogDisabledLevel, logrus.DebugLevel).
		ErrorStderrLines(1).
		Chroot(pmChroot.ChrootDir()).
		Execute()
}

func (pm *tdnfPackageManager) isPackageInstalled(imageChroot safechroot.ChrootInterface,
	toolsChroot *safechroot.Chroot, packageName string,
) (bool, error) {
	args := []string{"info", packageName, "--repo", "@system"}
	chroot := imageChroot
	if toolsChroot != nil {
		// Run tdnf from inside the tools chroot against the image bind-mounted at /_imageroot — needed when
		// imageChroot has no in-image tdnf (e.g. ACL).
		args = append([]string{
			"--releasever=" + pm.getReleaseVersion(),
			"--installroot=/" + toolsRootImageDir,
		}, args...)
		chroot = toolsChroot
	}

	err := shell.NewExecBuilder("tdnf", args...).
		LogLevel(logrus.TraceLevel, logrus.DebugLevel).
		Chroot(chroot.ChrootDir()).
		Execute()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// The command ran and failed.
			return false, nil
		}

		// The command failed to start.
		return false, err
	}

	return true, nil
}

func (pm *tdnfPackageManager) importGpgKeys(imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	chrootGpgKeys []string, uriGpgKeys []string,
) error {
	// tdnf doesn't do gpg import when downloading repo metadata, only when installing packages.
	// So, it has to be done manually. :-(

	if len(uriGpgKeys) > 0 {
		logger.Log.Infof("GPG import not implemented yet for remote URIs (%v)", uriGpgKeys)
	}

	if len(chrootGpgKeys) <= 0 {
		// No gpg keys to import.
		return nil
	}

	chroot := imageChroot
	if toolsChroot != nil {
		chroot = toolsChroot
	}

	for _, gpgKey := range chrootGpgKeys {
		err := shell.NewExecBuilder("gpg", "--import", gpgKey).
			LogLevel(logrus.DebugLevel, logrus.DebugLevel).
			ErrorStderrLines(2).
			Chroot(chroot.ChrootDir()).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to import GPG key (%s):\n%w", gpgKey, err)
		}
	}

	return nil
}
