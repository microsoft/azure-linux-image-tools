// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
	"time"
)

type PackageSnapshotTime string

func (p PackageSnapshotTime) IsValid() error {
	_, err := p.Parse()
	return err
}

func (p PackageSnapshotTime) Parse() (time.Time, error) {
	str := string(p)

	if str == "" {
		return time.Time{}, nil
	}

	// Try RFC3339
	if t, err := time.Parse(time.RFC3339, str); err == nil {
		return t, nil
	}

	// Try ISO 8601 (date only)
	if t, err := time.Parse("2006-01-02", str); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("invalid snapshot time format: must be YYYY-MM-DD or full RFC3339 (e.g., 2024-05-20T15:04:05Z)")
}
