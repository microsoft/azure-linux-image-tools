// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/userutils"
)

// Group holds information for a new user group.
type Group struct {
	// Name is the name of the group.
	Name string `yaml:"name" json:"name,omitempty"`
	// GID is the (optional) ID number to give to the new group.
	GID *int `yaml:"gid" json:"gid,omitempty"`
}

// IsValid returns an error if the MountPoint is not valid
func (g *Group) IsValid() error {
	err := userutils.NameIsValid(g.Name)
	if err != nil {
		return fmt.Errorf("group (%s) is invalid:\n%w", g.Name, err)
	}

	if g.GID != nil {
		err := userutils.UIDIsValid(*g.GID)
		if err != nil {
			return fmt.Errorf("group (%s) is invalid:\n%w", g.Name, err)
		}
	}

	return nil
}
