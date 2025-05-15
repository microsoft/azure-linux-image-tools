// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type PackageSnapshotTime string

func (p PackageSnapshotTime) IsValid() error {
	str := string(p)
	if str == "" {
		return errors.New("snapshot time cannot be empty")
	}

	parts := strings.Split(str, ":")
	if len(parts) != 3 {
		return fmt.Errorf("snapshot time must be in yyyy:mm:dd format (e.g. 2025:04:28)")
	}

	year, err := strconv.Atoi(parts[0])
	if err != nil || year < 2000 {
		return fmt.Errorf("invalid year in snapshot time: %s", parts[0])
	}

	month, err := strconv.Atoi(parts[1])
	if err != nil || month < 1 || month > 12 {
		return fmt.Errorf("invalid month in snapshot time: %s", parts[1])
	}

	day, err := strconv.Atoi(parts[2])
	if err != nil || day < 1 || day > 31 {
		return fmt.Errorf("invalid day in snapshot time: %s", parts[2])
	}

	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if date.Year() != year || int(date.Month()) != month || date.Day() != day {
		return fmt.Errorf("invalid calendar date: %s", str)
	}

	if date.After(time.Now().UTC()) {
		return fmt.Errorf("snapshot time is in the future: %s", str)
	}

	return nil
}
