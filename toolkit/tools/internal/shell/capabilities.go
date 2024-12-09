// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package shell

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/sliceutils"
	"golang.org/x/sys/unix"
)

func setOSThreadCapabilities(capabilities []uintptr) error {
	maxBoundingCapability, err := getMaxBoundingCapability()
	if err != nil {
		return fmt.Errorf("failed to get number of Linux capabilities:\n%w", err)
	}

	for i := uintptr(0); i <= maxBoundingCapability; i++ {
		keep := sliceutils.ContainsValue(capabilities, i)
		if keep {
			continue
		}

		enabled, err := readBoundingCapability(i)
		if err != nil {
			return fmt.Errorf("failed to read bounding capability state (%d)", i)
		}

		if !enabled {
			continue
		}

		err = dropBoundingCapability(i)
		if err != nil {
			return fmt.Errorf("failed to drop bounding capability (%d)", i)
		}
	}

	return nil
}

func getMaxBoundingCapability() (uintptr, error) {
	const lastCapFile = "/proc/sys/kernel/cap_last_cap"

	contentsBytes, err := os.ReadFile(lastCapFile)
	if err != nil {
		return 0, fmt.Errorf("failed to read cap_last_cap file:\n%w", err)
	}

	contents := strings.TrimSpace(string(contentsBytes))
	lastCap, err := strconv.Atoi(contents)
	if err != nil {
		return 0, fmt.Errorf("failed to parse cap_last_cap (%s):\n%w", contents, err)
	}

	return uintptr(lastCap), nil
}

func dropBoundingCapability(capability uintptr) error {
	err := unix.Prctl(unix.PR_CAPBSET_DROP, capability, 0, 0, 0)
	if err != nil {
		return fmt.Errorf("prctl failed:\n%w", err)
	}

	return nil
}

func readBoundingCapability(capability uintptr) (bool, error) {
	r, _, errno := unix.Syscall6(unix.SYS_PRCTL, unix.PR_CAPBSET_READ, capability, 0, 0, 0, 0)
	enabled := r != 0
	if errno != 0 {
		return enabled, errno
	}
	return enabled, nil
}
