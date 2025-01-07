// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package kernelversion

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/version"
)

var (
	// Parses the kernel version from "uname -r" or subdirectories of /lib/modules.
	//
	// Examples:
	//   OS               Version
	//   Fedora 40        6.11.6-200.fc40.x86_64
	//   Ubuntu 22.04     6.8.0-48-generic
	//   Azure Linux 2.0  5.15.153.1-2.cm2
	//   Azure Linux 3.0  6.6.47.1-1.azl3
	kernelVersionRegex = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)([.\-][a-zA-Z0-9_.\-]*)?$`)
)

func GetBuildHostKernelVersion() (version.Version, error) {
	stdout, _, err := shell.Execute("uname", "-r")
	if err != nil {
		return nil, fmt.Errorf("failed to get kernel version using uname:\n%w", err)
	}

	stdout = strings.TrimSpace(stdout)

	version, err := parseKernelVersion(stdout)
	if err != nil {
		return nil, err
	}

	return version, nil
}

func parseKernelVersion(versionString string) (version.Version, error) {
	match := kernelVersionRegex.FindStringSubmatch(versionString)
	if match == nil {
		return nil, fmt.Errorf("failed to parse kernel version (%s)", versionString)
	}

	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	patch, _ := strconv.Atoi(match[3])

	version := version.Version{major, minor, patch}
	return version, nil
}
