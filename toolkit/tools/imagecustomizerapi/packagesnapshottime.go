// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
	"time"
)

type PackageSnapshotTime string

func (p PackageSnapshotTime) IsValid() error {
	str := string(p)
	if str == "" {
		return nil
	}

	// Try full RFC 3339 first
	if _, err := time.Parse(time.RFC3339, str); err == nil {
		return nil
	}

	// Try date-only format
	if _, err := time.Parse("2006-01-02", str); err == nil {
		return nil
	}

	return fmt.Errorf("invalid snapshot time format: must be YYYY-MM-DD or full RFC3339 (e.g., 2024-05-20T15:04:05Z)")
}
