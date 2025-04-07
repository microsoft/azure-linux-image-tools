// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package processes

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/sirupsen/logrus"
)

var (
	// Example:
	//     revision: 4.93.2
	lsofVersionRegexp = regexp.MustCompile(`(?m)^\s*revision:\s+(\d+)\.(\d+)\.\d+\s*$`)
)

type ProcessRecord struct {
	ProcessId   int
	ProcessName string
	ProcessRoot string
}

// GetProcessesUsingPath returns a list of all the processes that have a file opened under the provided path.
func GetProcessesUsingPath(path string) ([]ProcessRecord, error) {
	lsofVersionMajor, lsofVersionMinor, err := getLsofVersion()
	if err != nil {
		return nil, err
	}

	qArgAvailable := lsofVersionMajor > 4 || (lsofVersionMajor == 4 && lsofVersionMinor >= 95)

	args := []string(nil)
	if qArgAvailable {
		args = append(args, "-Q")
	}

	args = append(args, "-F", "pcn", "--", path)

	stdout, _, err := shell.NewExecBuilder("lsof", args...).
		LogLevel(logrus.TraceLevel, logrus.DebugLevel).
		ExecuteCaptureOuput()
	if err != nil {
		if !qArgAvailable {
			// The -Q arg isn't available. So, this error could just mean there are no results.
			// So, just hope that this is in fact the case, to avoid unexpected errors.
			return nil, nil
		}

		return nil, fmt.Errorf("failed to list running processes using path (%s) using lsof\n%w", path,
			err)
	}

	records := []ProcessRecord(nil)
	record := ProcessRecord{
		ProcessId: -1,
	}
	scanner := bufio.NewScanner(strings.NewReader(stdout))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) <= 0 {
			continue
		}

		prefix := line[0]
		value := line[1:]
		switch prefix {
		case 'p':
			if record.ProcessId >= 0 {
				// Add previous item.
				records = append(records, record)
			}

			record = ProcessRecord{}

			record.ProcessId, err = strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("failed to parse process ID string (%s):\n%w", value, err)
			}

			record.ProcessRoot, err = os.Readlink(fmt.Sprintf("/proc/%d/root", record.ProcessId))
			if err != nil {
				return nil, fmt.Errorf("failed to read process chroot path (%d):\n%w", record.ProcessId, err)
			}

		case 'c':
			record.ProcessName = value

		case 'n':
			// 'n' is only requested so that it is in the trace logs.
		}
	}

	if record.ProcessId >= 0 {
		// Add last item.
		records = append(records, record)
	}

	return records, nil
}

func getLsofVersion() (int, int, error) {
	_, stderr, err := shell.NewExecBuilder("lsof", "-v").
		LogLevel(logrus.TraceLevel, logrus.TraceLevel).
		ExecuteCaptureOuput()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get lsof's version:\n%w", err)
	}

	match := lsofVersionRegexp.FindStringSubmatch(stderr)
	if match == nil {
		return 0, 0, fmt.Errorf("failed to parse lsof version string")
	}

	majorStr := match[1]
	minorStr := match[2]

	major, _ := strconv.Atoi(majorStr)
	minor, _ := strconv.Atoi(minorStr)

	return major, minor, nil
}

func StopProcessById(pid int) error {
	logger.Log.Debugf("Stopping process: Pid=%d.", pid)

	process, err := os.FindProcess(pid)
	defer process.Release()
	if err != nil {
		return fmt.Errorf("failed to find process (%d) to stop:\n%w", pid, err)
	}

	err = process.Signal(os.Interrupt)
	if err != nil {
		return fmt.Errorf("failed to stop process (%d):\n%w", pid, err)
	}

	return nil
}
