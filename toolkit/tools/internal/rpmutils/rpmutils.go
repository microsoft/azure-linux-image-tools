// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package rpmutils

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	// NAME-[EPOCH:]VERSION-RELEASE.ARCH
	// Note: Greedy is required since the values can contain periods (.) and dashes (-).
	rpmNevraRegex = regexp.MustCompile(`^(\S+)-((\d+):)?(\S+)-(\S+)\.(\S+)$`)

	// NAME-[EPOCH:]VERSION-RELEASE.ARCH
	// Note: Greedy is required since the values can contain periods (.) and dashes (-).
	rpmEvrRegex = regexp.MustCompile(`^((\d+):)?(\S+)-(\S+)$`)

	ErrPackagesInvalidRpmNevra = errors.New("invalid RPM NEVRA format")
	ErrPackagesInvalidRpmEvr   = errors.New("invalid RPM EVR format")
)

func Evr(epoch, version, release string) string {
	evr := ""
	if epoch != "" {
		evr += epoch + ":"
	}
	evr += version + "-" + release

	return evr
}

func ParseNevra(nevra string) (string, string, string, string, string, error) {
	match := rpmNevraRegex.FindStringSubmatch(nevra)
	if match == nil {
		return "", "", "", "", "", fmt.Errorf("%w (%s)", ErrPackagesInvalidRpmNevra, nevra)
	}

	name := match[1]
	epoch := match[3]
	version := match[4]
	release := match[5]
	arch := match[6]

	return name, epoch, version, release, arch, nil
}

func ParseEvr(evr string) (string, string, string, error) {
	match := rpmEvrRegex.FindStringSubmatch(evr)
	if match == nil {
		return "", "", "", fmt.Errorf("%w (%s)", ErrPackagesInvalidRpmEvr, evr)
	}

	epoch := match[2]
	version := match[3]
	release := match[4]

	return epoch, version, release, nil
}
