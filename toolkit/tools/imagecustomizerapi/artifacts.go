// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import "fmt"

type Artifacts struct {
	Items []Item `yaml:"items" json:"items,omitempty"`
	Path  string `yaml:"path" json:"path,omitempty"`
}

func (a Artifacts) IsValid() error {
	if (len(a.Items) == 0 && a.Path != "") || (len(a.Items) > 0 && a.Path == "") {
		return fmt.Errorf("'items' and 'path' should either both be provided or neither")
	}

	for _, item := range a.Items {
		err := item.IsValid()
		if err != nil {
			return err
		}
	}

	if err := validatePath(a.Path); err != nil {
		return fmt.Errorf("invalid 'path' field:\n%w", err)
	}

	return nil
}
