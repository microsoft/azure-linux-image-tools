// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import "fmt"

type Artifacts struct {
	Items []OutputArtifactsItemType `yaml:"items" json:"items,omitempty"`
	Path  string                    `yaml:"path" json:"path,omitempty"`
}

func (a *Artifacts) IsValid() error {
	if len(a.Items) == 0 || a.Path == "" {
		return fmt.Errorf("'items' and 'path' must both be specified and non-empty")
	}

	for _, item := range a.Items {
		if err := item.IsValid(); err != nil {
			return err
		}
	}

	return nil
}
