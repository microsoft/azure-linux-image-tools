// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package processes

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/sirupsen/logrus"
)

type ProcessRecord struct {
	ProcessId   int
	ProcessName string
	ProcessRoot string
}

// GetProcessesUsingPath returns a list of all the processes that have a file opened under the provided path.
func GetProcessesUsingPath(path string) ([]ProcessRecord, error) {
	stdout, _, err := shell.NewExecBuilder("lsof", "-Q", "-F", "pcn", "--", path).
		LogLevel(logrus.TraceLevel, logrus.DebugLevel).
		ExecuteCaptureOuput()
	if err != nil {
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
		if len(line) < 0 {
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
