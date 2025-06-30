// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package userutils

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/randomization"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/sirupsen/logrus"
)

const (
	RootUser          = "root"
	RootHomeDir       = "/root"
	UserHomeDirPrefix = "/home"

	ShadowFile                = "/etc/shadow"
	PasswdFile                = "/etc/passwd"
	GroupFile                 = "/etc/group"
	SSHDirectoryName          = ".ssh"
	SSHAuthorizedKeysFileName = "authorized_keys"

	SshDirectoryPerm   os.FileMode = 0o700
	AuthorizedKeysPerm os.FileMode = 0o600
)

func HashPassword(password string) (string, error) {
	const postfixLength = 12

	if password == "" {
		return "", nil
	}

	salt, err := randomization.RandomString(postfixLength, randomization.LegalCharactersAlphaNum)
	if err != nil {
		return "", fmt.Errorf("failed to generate salt for hashed password:\n%w", err)
	}

	// Generate hashed password based on salt value provided.
	// -6 option indicates to use the SHA256/SHA512 algorithm
	stdout, _, err := shell.NewExecBuilder("openssl", "passwd", "-6", "-salt", salt, "-stdin").
		Stdin(password).
		LogLevel(shell.LogDisabledLevel, logrus.DebugLevel).
		ExecuteCaptureOutput()
	if err != nil {
		return "", fmt.Errorf("failed to generate hashed password:\n%w", err)
	}

	hashedPassword := strings.TrimSpace(stdout)
	return hashedPassword, nil
}

func UserExists(username string, installChroot safechroot.ChrootInterface) (bool, error) {
	var userExists bool
	err := installChroot.UnsafeRun(func() error {
		_, stderr, err := shell.Execute("id", "-u", username)
		if err != nil {
			if !strings.Contains(stderr, "no such user") {
				return fmt.Errorf("failed to check if user exists (%s):\n%w", username, err)
			}

			userExists = false
		} else {
			userExists = true
		}

		return nil
	})
	if err != nil {
		return false, err
	}

	return userExists, nil
}

func AddUser(username string, homeDir string, primaryGroup string, hashedPassword string, uid string, installChroot safechroot.ChrootInterface) error {
	var args = []string{username, "-m"}
	if hashedPassword != "" {
		args = append(args, "-p", hashedPassword)
	}
	if uid != "" {
		args = append(args, "-u", uid)
	}
	if homeDir != "" {
		args = append(args, "-d", homeDir)
	}
	if primaryGroup != "" {
		args = append(args, "-g", primaryGroup)
	}

	err := installChroot.UnsafeRun(func() error {
		return shell.ExecuteLiveWithErr(1, "useradd", args...)
	})
	if err != nil {
		return fmt.Errorf("failed to add user (%s):\n%w", username, err)
	}

	return nil
}

func UpdateUserPassword(installRoot, username, hashedPassword string) error {
	shadowFilePath := filepath.Join(installRoot, ShadowFile)

	if hashedPassword == "" {
		// In the /etc/shadow file, the values `*` and `!` both mean the user's password login is disabled but the user
		// may login using other means (e.g. ssh, auto-login, etc.). This interpretation is also used by PAM. When sshd
		// has `UsePAM` set to `yes`, then sshd defers to PAM the decision on whether or not the user is disabled.
		// However, when `UsePAM` is set to `no`, then sshd must make this interpretation for itself. And the Azure Linux
		// build of sshd is configured to interpret the `!` in the shadow file to mean the user is fully disabled, even
		// for ssh login. But it interprets `*` to mean that only password login is disabled but sshd public/private key
		// login is fine.
		hashedPassword = "*"
	}

	// Find the line that starts with "<user>:<password>:..."
	findUserEntry, err := regexp.Compile(fmt.Sprintf("(?m)^%s:[^:]*:", regexp.QuoteMeta(username)))
	if err != nil {
		return fmt.Errorf("failed to compile user (%s) password update regex:\n%w", username, err)
	}

	// Read in existing /etc/shadow file.
	shadowFileBytes, err := os.ReadFile(shadowFilePath)
	if err != nil {
		return fmt.Errorf("failed to read shadow file (%s) to update user's (%s) password:\n%w", shadowFilePath, username, err)
	}

	shadowFile := string(shadowFileBytes)

	// Try to find the user's entry.
	entryIndexes := findUserEntry.FindStringIndex(shadowFile)
	if entryIndexes == nil {
		return fmt.Errorf("failed to find user (%s) in shadow file (%s)", username, shadowFilePath)
	}

	newShadowFile := fmt.Sprintf("%s%s:%s:%s", shadowFile[:entryIndexes[0]], username, hashedPassword, shadowFile[entryIndexes[1]:])

	// Write new /etc/shadow file.
	err = file.Write(newShadowFile, shadowFilePath)
	if err != nil {
		return fmt.Errorf("failed to write new shadow file (%s) to update user's (%s) password:\n%w", shadowFilePath, username, err)
	}

	return nil
}

// UserHomeDirectory returns the home directory for a user.
func UserHomeDirectory(installRoot string, username string) (string, error) {
	entry, err := GetPasswdFileEntryForUser(installRoot, username)
	if err != nil {
		return "", err
	}

	return entry.HomeDirectory, nil
}

// UserSSHDirectory returns the path of the .ssh directory for a user.
func UserSSHDirectory(installRoot string, username string) (string, error) {
	homeDir, err := UserHomeDirectory(installRoot, username)
	if err != nil {
		return "", err
	}

	userSSHKeyDir := filepath.Join(homeDir, SSHDirectoryName)
	return userSSHKeyDir, nil
}

// NameIsValid returns an error if the User name is empty
func NameIsValid(name string) (err error) {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("invalid value for name (%s), name cannot be empty", name)
	}
	return
}

// UIDIsValid returns an error if the UID is outside bounds
// UIDs 1-999 are system users and 1000-60000 are normal users
// Bounds can be checked using:
// $grep -E '^UID_MIN|^UID_MAX' /etc/login.defs
func UIDIsValid(uid int) error {
	const (
		uidLowerBound = 0 // root user
		uidUpperBound = 60000
	)

	if uid < uidLowerBound || uid > uidUpperBound {
		return fmt.Errorf("invalid value for UID (%d), not within [%d, %d]", uid, uidLowerBound, uidUpperBound)
	}

	return nil
}

// PasswordExpiresDaysISValid returns an error if the expire days is not
// within bounds set by the chage -M command
func PasswordExpiresDaysIsValid(passwordExpiresDays int64) error {
	const (
		noExpiration    = -1 //no expiration
		upperBoundChage = 99999
	)
	if passwordExpiresDays < noExpiration || passwordExpiresDays > upperBoundChage {
		return fmt.Errorf("invalid value for PasswordExpiresDays (%d), not within [%d, %d]", passwordExpiresDays, noExpiration, upperBoundChage)
	}
	return nil
}

func GetUserId(username string, installChroot safechroot.ChrootInterface) (int, error) {
	var stdout string
	err := installChroot.UnsafeRun(func() error {
		var err error
		stdout, _, err = shell.NewExecBuilder("id", "-u", username).
			ErrorStderrLines(1).
			LogLevel(logrus.DebugLevel, logrus.DebugLevel).
			ExecuteCaptureOutput()
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("failed to get user's (%s) ID:\n%w", username, err)
	}

	id, err := strconv.Atoi(strings.TrimSpace(stdout))
	if err != nil {
		return 0, fmt.Errorf("failed to parse user's (%s) ID (%s):\n%w", username, stdout, err)
	}

	return id, nil
}

func GetUserGroupId(username string, installChroot safechroot.ChrootInterface) (int, error) {
	var stdout string
	err := installChroot.UnsafeRun(func() error {
		var err error
		stdout, _, err = shell.NewExecBuilder("id", "-g", username).
			ErrorStderrLines(1).
			LogLevel(logrus.DebugLevel, logrus.DebugLevel).
			ExecuteCaptureOutput()
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("failed to get user's (%s) group ID:\n%w", username, err)
	}

	id, err := strconv.Atoi(strings.TrimSpace(stdout))
	if err != nil {
		return 0, fmt.Errorf("failed to parse user's (%s) group ID (%s):\n%w", username, stdout, err)
	}

	return id, nil
}

func ProvisionUserSSHCerts(installChroot safechroot.ChrootInterface, username string, sshPubKeyPaths []string,
	sshPubKeys []string, includeExistingKeys bool,
) (err error) {
	// Skip user SSH directory generation when not provided with public keys
	// Let SSH handle the creation of this folder on its first use
	if len(sshPubKeyPaths) == 0 && len(sshPubKeys) == 0 {
		return
	}

	userSSHKeyDir, err := UserSSHDirectory(installChroot.RootDir(), username)
	if err != nil {
		return fmt.Errorf("failed to get user's (%s) home directory:\n%w", username, err)
	}

	userSSHKeyDirFullPath := filepath.Join(installChroot.RootDir(), userSSHKeyDir)
	authorizedKeysFullPath := filepath.Join(userSSHKeyDirFullPath, SSHAuthorizedKeysFileName)

	allSSHKeys := []string(nil)

	// Add existing keys in the authorized_keys file, if requested.
	if includeExistingKeys {
		fileExists, err := file.PathExists(authorizedKeysFullPath)
		if err != nil {
			return fmt.Errorf("failed to check if authorized_keys file (%s) exists:\n%w", authorizedKeysFullPath, err)
		}

		if fileExists {
			pubKeyData, err := file.ReadLines(authorizedKeysFullPath)
			if err != nil {
				return fmt.Errorf("failed to read existing authorized_keys (%s) file:\n%w", authorizedKeysFullPath, err)
			}

			allSSHKeys = append(allSSHKeys, pubKeyData...)
		}
	}

	// Add SSH keys from sshPubKeyPaths
	for _, pubKey := range sshPubKeyPaths {
		pubKeyData, err := file.ReadLines(pubKey)
		if err != nil {
			return fmt.Errorf("failed to read SSH public key file (%s):\n%w", pubKey, err)
		}

		allSSHKeys = append(allSSHKeys, pubKeyData...)
	}

	// Add direct SSH keys
	allSSHKeys = append(allSSHKeys, sshPubKeys...)

	// Get user's IDs.
	uid, err := GetUserId(username, installChroot)
	if err != nil {
		return err
	}

	gid, err := GetUserGroupId(username, installChroot)
	if err != nil {
		return err
	}

	// Create the .ssh directory, if needed.
	sshKeyDirExists, err := file.PathExists(userSSHKeyDirFullPath)
	if err != nil {
		return fmt.Errorf("failed to check if user's .ssh directory (%s) exists:\n%w", userSSHKeyDirFullPath, err)
	}

	if !sshKeyDirExists {
		err = os.Mkdir(userSSHKeyDirFullPath, SshDirectoryPerm)
		if err != nil {
			return fmt.Errorf("failed to create user's .ssh directory (%s):\n%w", userSSHKeyDirFullPath, err)
		}

		// Reapply the permissions to avoid the umask changing the value.
		err = os.Chmod(userSSHKeyDirFullPath, SshDirectoryPerm)
		if err != nil {
			return fmt.Errorf("failed to set permissions on user's .ssh directory (%s):\n%w", userSSHKeyDirFullPath, err)
		}

		err := os.Chown(userSSHKeyDirFullPath, uid, gid)
		if err != nil {
			return fmt.Errorf("failed to set ownership on user's .ssh directory (%s):\n%w", userSSHKeyDirFullPath, err)
		}
	}

	// Create the authorized_keys file.
	authorizedKeysFile, err := os.OpenFile(authorizedKeysFullPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, AuthorizedKeysPerm)
	if err != nil {
		return fmt.Errorf("failed to create authorized_keys file (%s):\n%w", authorizedKeysFullPath, err)
	}
	defer authorizedKeysFile.Close()

	// Reapply the permissions to avoid the umask changing the value.
	err = authorizedKeysFile.Chmod(AuthorizedKeysPerm)
	if err != nil {
		return fmt.Errorf("failed to set authorized_keys file (%s) permission:\n%w", authorizedKeysFullPath, err)
	}

	err = authorizedKeysFile.Chown(uid, gid)
	if err != nil {
		return fmt.Errorf("failed to set authorized_keys file (%s) permission:\n%w", authorizedKeysFullPath, err)
	}

	// Write SSH keys.
	for _, pubKey := range allSSHKeys {
		logger.Log.Infof("Adding ssh key (%s) to user (%s)", filepath.Base(pubKey), username)

		pubKeyLine := pubKey + "\n"
		_, err := authorizedKeysFile.WriteString(pubKeyLine)
		if err != nil {
			return fmt.Errorf("failed to write authorized_keys file (%s) line:\n%w", authorizedKeysFullPath, err)
		}
	}

	err = authorizedKeysFile.Close()
	if err != nil {
		return fmt.Errorf("failed to close authorized_keys file (%s):\n%w", authorizedKeysFullPath, err)
	}

	return
}
