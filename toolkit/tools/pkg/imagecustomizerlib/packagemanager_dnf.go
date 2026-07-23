// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"errors"
	"os/exec"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/sirupsen/logrus"
)

// DNF Package Manager Implementation
type dnfPackageManager struct {
	version string
}

func newDnfPackageManager(version string) *dnfPackageManager {
	return &dnfPackageManager{version: version}
}

func (pm *dnfPackageManager) getReleaseVersion() string { return pm.version }

// getCacheOnlyOptions returns DNF-specific cache options for install/update operations
func (pm *dnfPackageManager) getCacheOnlyOptions() []string {
	return []string{"--setopt=cacheonly=metadata"}
}

func (pm *dnfPackageManager) configureSnapshotTime(packageManagerChroot *safechroot.Chroot,
	snapshotTime imagecustomizerapi.PackageSnapshotTime,
) (func() error, error) {
	return nil, fmt.Errorf("%w:\npackage manager dnf does not support snapshot time", ErrSnapshotTimeNotSupported)
}

func (pm *dnfPackageManager) executeCommand(args []string, imageChroot *safechroot.Chroot,
	toolsChroot *safechroot.Chroot,
) error {
	pmChroot := imageChroot
	if toolsChroot != nil {
		pmChroot = toolsChroot
	}

	seenTransactionErrorMessage := false
	stderrCallback := func(line string) {
		switch {
		case line == "Failed to resolve the transaction:" || strings.HasPrefix(line, "Transaction failed:"):
			seenTransactionErrorMessage = true
			fallthrough

		case seenTransactionErrorMessage:
			if line == "" {
				logger.Log.Debug(line)
			} else {
				logger.Log.Warn(line)
			}

		default:
			logger.Log.Debug(line)
		}
	}

	return shell.NewExecBuilder(packageManagerDNF, args...).
		LogLevel(logrus.DebugLevel, shell.LogDisabledLevel).
		StderrCallback(stderrCallback).
		Chroot(pmChroot.ChrootDir()).
		Execute()
}

func (pm *dnfPackageManager) isPackageInstalled(imageChroot safechroot.ChrootInterface,
	toolsChroot *safechroot.Chroot, packageName string,
) (bool, error) {
	// Use `rpm -q` rather than `dnf info --installed` here: it queries the local rpm database directly without
	// opening any log files for writing, so it works on read-only chroots and avoids the security concerns of
	// pointing dnf's logdir at a fixed, predictable path inside the chroot. `rpm` is guaranteed to be present in any
	// chroot that has dnf5 installed because dnf5 takes a hard dependency on rpm.
	args := []string{"-q", "--", packageName}
	chroot := imageChroot
	if toolsChroot != nil {
		// Run rpm from inside the tools chroot against the image bind-mounted at /_imageroot — needed when
		// imageChroot has no in-image rpm.
		args = append([]string{"--root", "/" + toolsRootImageDir}, args...)
		chroot = toolsChroot
	}

	err := shell.NewExecBuilder("rpm", args...).
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

func (pm *dnfPackageManager) importGpgKeys(imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	gpgKeys []string, uriGpgKeys []string,
) error {
	// dnf handles gpg import automatically.
	// So, nothing to do.
	return nil
}
