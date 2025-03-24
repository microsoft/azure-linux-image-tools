// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import "fmt"

type Artifacts struct {
	Items []OutputArtifactsItemType `yaml:"items" json:"items,omitempty"`
	Path  string                    `yaml:"path" json:"path,omitempty"`
}

func (a Artifacts) IsValid() error {
	if len(a.Items) == 0 || a.Path == "" {
		return fmt.Errorf("'items' and 'path' must both be specified and non-empty")
	}

	for _, item := range a.Items {
		err := item.IsValid()
		if err != nil {
			return err
		}
	}

	if err := validatePathWithAbs(a.Path, false); err != nil {
		return fmt.Errorf("invalid 'path' field:\n%w", err)
	}

	return nil
}
