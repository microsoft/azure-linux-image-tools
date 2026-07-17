// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"slices"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/packagemanifestapi"
	"github.com/sirupsen/logrus"
)

var (
	// Example:
	//
	// Package                     Arch   Version         Repository    Size
	// Upgrading:
	//  NetworkManager             x86_64 1:1.52.2-1.fc42 updates    5.8 MiB
	//    replacing NetworkManager x86_64 1:1.52.0-1.fc42 updates    5.8 MiB
	dnfTableEntryRow      = regexp.MustCompile(`^\s+(replacing\s+)?(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+\s+\S+)$`)
	dnfTableHeaderInstall = []string{
		"Installing weak dependencies:",
		"Installing group/module packages:",
		"Installing:",
		"Upgrading:",
		"Downgrading:",
		"Reinstalling:",
	}
	dnfTableHeaderRemove = []string{
		"Removing dependent packages:",
		"Removing unused dependencies:",
		"Removing:",
	}
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
) ([]packagemanifestapi.Package, []packagemanifestapi.Package, error) {
	pmChroot := imageChroot
	if toolsChroot != nil {
		pmChroot = toolsChroot
	}

	fullArgs := []string{}
	fullArgs = append(fullArgs, args...)

	installedPackages := []packagemanifestapi.Package(nil)
	removedPackages := []packagemanifestapi.Package(nil)
	seenTransactionErrorMessage := false

	type stdoutState int
	const (
		stdoutStateStart stdoutState = iota
		stdoutStateInstall
		stdoutStateRemove
		stdoutStateEnd
	)

	currStdoutState := stdoutStateStart
	stdoutCallback := func(line string) {
		if currStdoutState == stdoutStateEnd {
			return
		}

		switch {
		case line == "":
			currStdoutState = stdoutStateEnd

		case slices.Contains(dnfTableHeaderInstall, line):
			currStdoutState = stdoutStateInstall

		case slices.Contains(dnfTableHeaderRemove, line):
			currStdoutState = stdoutStateRemove

		default:
			match := dnfTableEntryRow.FindStringSubmatch(line)
			if match == nil {
				return
			}

			replacing := match[1]
			name := match[2]
			arch := match[3]
			version := match[4]

			packageInfo := packagemanifestapi.Package{
				Type:    packagemanifestapi.PackageTypeRpm,
				Name:    name,
				Version: version,
				Arch:    arch,
			}

			if replacing != "" || currStdoutState == stdoutStateInstall {
				installedPackages = append(installedPackages, packageInfo)
			} else {
				removedPackages = append(removedPackages, packageInfo)
			}
		}
	}

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

	err := shell.NewExecBuilder(packageManagerDNF, fullArgs...).
		LogLevel(logrus.DebugLevel, shell.LogDisabledLevel).
		StdoutCallback(stdoutCallback).
		StderrCallback(stderrCallback).
		Chroot(pmChroot.ChrootDir()).
		Execute()
	if err != nil {
		return nil, nil, err
	}

	return installedPackages, removedPackages, nil
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

func (pm *dnfPackageManager) getPackageInformation(imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	packageName string,
) (*PackageVersionInformation, error) {
	// Use `rpm -q --queryformat` rather than `dnf info --installed` here for the same reason as in isPackageInstalled.
	//
	// Use `--queryformat` to get a single-line, parser-friendly output that matches what parsePackageInfoOutput
	// already expects from `tdnf info` (Name/Version/Release labels), so we share the parser.
	args := []string{"-q", "--queryformat",
		"Name : %{NAME}\nVersion : %{VERSION}\nRelease : %{RELEASE}\n", "--", packageName}
	chroot := imageChroot
	if toolsChroot != nil {
		// Run rpm from inside the tools chroot against the image bind-mounted at /_imageroot — needed when
		// imageChroot has no in-image rpm.
		args = append([]string{"--root", "/" + toolsRootImageDir}, args...)
		chroot = toolsChroot
	}

	packageInfo, _, err := shell.NewExecBuilder("rpm", args...).
		LogLevel(logrus.TraceLevel, logrus.DebugLevel).
		Chroot(chroot.ChrootDir()).
		ExecuteCaptureOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to query (%s) package information via rpm:\n%w", packageName, err)
	}

	info, err := parsePackageInfoOutput(packageName, packageInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to parse (%s) package information from rpm:\n%w", packageName, err)
	}
	return info, nil
}

func (pm *dnfPackageManager) importGpgKeys(imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	gpgKeys []string, uriGpgKeys []string,
) error {
	// dnf handles gpg import automatically.
	// So, nothing to do.
	return nil
}
