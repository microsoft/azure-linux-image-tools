// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/userutils"
)

type User struct {
	Name                string    `yaml:"name" json:"name,omitempty"`
	UID                 *int      `yaml:"uid" json:"uid,omitempty"`
	Password            *Password `yaml:"password" json:"password,omitempty"`
	PasswordExpiresDays *int64    `yaml:"passwordExpiresDays" json:"passwordExpiresDays,omitempty"`
	SSHPublicKeyPaths   []string  `yaml:"sshPublicKeyPaths" json:"sshPublicKeyPaths,omitempty"`
	SSHPublicKeys       []string  `yaml:"sshPublicKeys" json:"sshPublicKeys,omitempty"`
	PrimaryGroup        string    `yaml:"primaryGroup" json:"primaryGroup,omitempty"`
	SecondaryGroups     []string  `yaml:"secondaryGroups" json:"secondaryGroups,omitempty"`
	StartupCommand      string    `yaml:"startupCommand" json:"startupCommand,omitempty"`
	HomeDirectory       string    `yaml:"homeDirectory" json:"homeDirectory,omitempty"`
}

func (u *User) IsValid() error {
	err := userutils.NameIsValid(u.Name)
	if err != nil {
		return fmt.Errorf("user (%s) is invalid:\n%w", u.Name, err)
	}

	if u.UID != nil {
		err := userutils.UIDIsValid(*u.UID)
		if err != nil {
			return fmt.Errorf("user (%s) is invalid:\n%w", u.Name, err)
		}
	}

	if u.Password != nil {
		err := u.Password.IsValid()
		if err != nil {
			return fmt.Errorf("user (%s) is invalid:\n%w", u.Name, err)
		}
	}

	if u.PasswordExpiresDays != nil {
		err := userutils.PasswordExpiresDaysIsValid(*u.PasswordExpiresDays)
		if err != nil {
			return fmt.Errorf("user (%s) is invalid:\n%w", u.Name, err)
		}
	}

	return nil
}
