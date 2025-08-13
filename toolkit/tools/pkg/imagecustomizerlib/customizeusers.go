// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/installutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/userutils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var (
	// User management errors
	ErrUserExistsCheck                = NewImageCustomizerError("Users:ExistsCheck", "failed to check if user exists")
	ErrUserPasswordFileRead           = NewImageCustomizerError("Users:PasswordFileRead", "failed to read password file")
	ErrUserPasswordHash               = NewImageCustomizerError("Users:PasswordHash", "failed to hash password")
	ErrUserCannotSetUidOnExisting     = NewImageCustomizerError("Users:CannotSetUidOnExisting", "cannot set UID on a user that already exists")
	ErrUserCannotSetHomeDirOnExisting = NewImageCustomizerError("Users:CannotSetHomeDirOnExisting", "cannot set home directory on a user that already exists")
	ErrUserUpdate                     = NewImageCustomizerError("Users:Update", "failed to update user")
	ErrUserAdd                        = NewImageCustomizerError("Users:Add", "failed to add user")
)

func AddOrUpdateUsers(ctx context.Context, users []imagecustomizerapi.User, baseConfigPath string, imageChroot safechroot.ChrootInterface) error {
	if len(users) == 0 {
		return nil
	}
	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "add_or_update_users")
	span.SetAttributes(
		attribute.Int("users_count", len(users)),
	)
	defer span.End()
	for _, user := range users {
		err := addOrUpdateUser(user, baseConfigPath, imageChroot)
		if err != nil {
			return err
		}
	}

	return nil
}

func addOrUpdateUser(user imagecustomizerapi.User, baseConfigPath string, imageChroot safechroot.ChrootInterface) error {
	// Check if the user already exists.
	userExists, err := userutils.UserExists(user.Name, imageChroot)
	if err != nil {
		return fmt.Errorf("%w (user='%s'):\n%w", ErrUserExistsCheck, user.Name, err)
	}

	if userExists {
		logger.Log.Infof("Updating user (%s)", user.Name)
	} else {
		logger.Log.Infof("Adding user (%s)", user.Name)
	}

	hashedPassword := ""
	shouldUpdatePassword := false

	if user.Password != nil {
		shouldUpdatePassword = true

		passwordIsFile := user.Password.Type == imagecustomizerapi.PasswordTypePlainTextFile ||
			user.Password.Type == imagecustomizerapi.PasswordTypeHashedFile

		passwordIsHashed := user.Password.Type == imagecustomizerapi.PasswordTypeHashed ||
			user.Password.Type == imagecustomizerapi.PasswordTypeHashedFile

		password := user.Password.Value
		if passwordIsFile {
			// Read password from file.
			passwordFullPath := file.GetAbsPathWithBase(baseConfigPath, user.Password.Value)

			passwordFileContents, err := os.ReadFile(passwordFullPath)
			if err != nil {
				return fmt.Errorf("%w (file='%s'):\n%w", ErrUserPasswordFileRead, passwordFullPath, err)
			}

			password = string(passwordFileContents)
		}

		hashedPassword = password
		if !passwordIsHashed {
			// Hash the password.
			hashedPassword, err = userutils.HashPassword(password)
			if err != nil {
				return fmt.Errorf("%w (user='%s'):\n%w", ErrUserPasswordHash, user.Name, err)
			}
		}
	}

	if userExists {
		if user.UID != nil {
			return fmt.Errorf("%w (UID='%d', user='%s')", ErrUserCannotSetUidOnExisting, *user.UID, user.Name)
		}

		if user.HomeDirectory != "" {
			return fmt.Errorf("%w (homeDir='%s', user='%s')", ErrUserCannotSetHomeDirOnExisting, user.HomeDirectory, user.Name)
		}

		// Only update password if explicitly provided
		if shouldUpdatePassword {
			err = userutils.UpdateUserPassword(imageChroot.RootDir(), user.Name, hashedPassword)
			if err != nil {
				return fmt.Errorf("%w (user='%s'):\n%w", ErrUserUpdate, user.Name, err)
			}
		}
	} else {
		var uidStr string
		if user.UID != nil {
			uidStr = strconv.Itoa(*user.UID)
		}

		// Add the user.
		err = userutils.AddUser(user.Name, user.HomeDirectory, user.PrimaryGroup, hashedPassword, uidStr, imageChroot)
		if err != nil {
			return fmt.Errorf("%w (user='%s'):\n%w", ErrUserAdd, user.Name, err)
		}
	}

	// Set user's password expiry.
	if user.PasswordExpiresDays != nil {
		err = installutils.Chage(imageChroot, *user.PasswordExpiresDays, user.Name)
		if err != nil {
			return fmt.Errorf("%w (user='%s'):\n%w", ErrUserUpdate, user.Name, err)
		}
	}

	// Update an existing user's primary group. A new user's primary group will have already been set by AddUser().
	if userExists {
		err = installutils.ConfigureUserPrimaryGroupMembership(imageChroot, user.Name, user.PrimaryGroup)
		if err != nil {
			return err
		}
	}
	// Set user's secondary groups.
	err = installutils.ConfigureUserSecondaryGroupMembership(imageChroot, user.Name, user.SecondaryGroups)
	if err != nil {
		return err
	}

	// Set user's SSH keys.
	for i, _ := range user.SSHPublicKeyPaths {
		user.SSHPublicKeyPaths[i] = file.GetAbsPathWithBase(baseConfigPath, user.SSHPublicKeyPaths[i])
	}

	err = userutils.ProvisionUserSSHCerts(imageChroot, user.Name, user.SSHPublicKeyPaths, user.SSHPublicKeys,
		userExists)
	if err != nil {
		return err
	}

	// Set user's startup command.
	err = installutils.ConfigureUserStartupCommand(imageChroot, user.Name, user.StartupCommand)
	if err != nil {
		return err
	}

	return nil
}
