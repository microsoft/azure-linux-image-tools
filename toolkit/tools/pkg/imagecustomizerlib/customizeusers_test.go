// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/installutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/ptrutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/userutils"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var (
	// Parses the password field in the /etc/shadow file, extracting the rounds count and the salt.
	shadowPasswordRegexp = regexp.MustCompile(`^\$([a-zA-Z0-9]*)\$((rounds=[0-9]+\$)?[a-zA-Z0-9./]*)\$[a-zA-Z0-9./]*$`)
)

func TestCustomizeImageUsers(t *testing.T) {
	baseImage, _ := checkSkipForCustomizeDefaultImage(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageUsers")
	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	rootSshPublicKey := "fake-root-public-key"

	test2Uid := 10042
	test2SshPublicKey := "fake-test-public-key"
	test2SshPublicKeyPath := "files/a.txt"
	test2PlainText := "cat"
	test2HomeDirectory := "/home/10042"
	test2StartupCommand := "/sbin/nologin"
	test2PasswordExpiresDays := int64(10)

	config := imagecustomizerapi.Config{
		OS: &imagecustomizerapi.OS{
			Users: []imagecustomizerapi.User{
				{
					Name: "root",
					SSHPublicKeys: []string{
						rootSshPublicKey,
					},
				},
				{
					Name: "test1",
				},
				{
					Name: "test2",
					UID:  &test2Uid,
					Password: &imagecustomizerapi.Password{
						Type:  "plain-text",
						Value: test2PlainText,
					},
					PasswordExpiresDays: &test2PasswordExpiresDays,
					SSHPublicKeys: []string{
						test2SshPublicKey,
					},
					SSHPublicKeyPaths: []string{
						test2SshPublicKeyPath,
					},
					SecondaryGroups: []string{
						"sudo",
					},
					StartupCommand: test2StartupCommand,
					HomeDirectory:  test2HomeDirectory,
				},
			},
		},
	}

	// Customize image.
	err := CustomizeImage(t.Context(), buildDir, testDir, &config, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err := connectToCoreEfiImage(buildDir, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Verify root user.
	verifySshAuthorizedKeys(t, imageConnection.Chroot().RootDir(), "/root", []string{rootSshPublicKey}, 0, 0)

	rootPasswdEntry, err := userutils.GetPasswdFileEntryForUser(imageConnection.Chroot().RootDir(), "root")
	if assert.NoError(t, err) {
		assert.Equal(t, 0, rootPasswdEntry.Uid)
		assert.Equal(t, 0, rootPasswdEntry.Gid)
		assert.Equal(t, "/root", rootPasswdEntry.HomeDirectory)
		assert.Equal(t, "/bin/bash", rootPasswdEntry.Shell)
	}

	rootUserGroups, err := userutils.GetUserGroups(imageConnection.Chroot().RootDir(), "root")
	if assert.NoError(t, err) {
		assert.ElementsMatch(t, rootUserGroups, []string{})
	}

	// Verify test1 user.
	test1PasswdEntry, err := userutils.GetPasswdFileEntryForUser(imageConnection.Chroot().RootDir(), "test1")
	if assert.NoError(t, err) {
		assert.Equal(t, "/home/test1", test1PasswdEntry.HomeDirectory)
		assert.Equal(t, "/bin/bash", test1PasswdEntry.Shell)
	}

	test1UserGroups, err := userutils.GetUserGroups(imageConnection.Chroot().RootDir(), "test1")
	if assert.NoError(t, err) {
		assert.ElementsMatch(t, test1UserGroups, []string{})
	}

	// Verify test2 user.
	verifySshAuthorizedKeys(t, imageConnection.Chroot().RootDir(), test2HomeDirectory,
		[]string{test2SshPublicKey, "abcdefg"}, test2Uid, test2Uid)

	test2PasswdEntry, err := userutils.GetPasswdFileEntryForUser(imageConnection.Chroot().RootDir(), "test2")
	if assert.NoError(t, err) {
		assert.Equal(t, test2Uid, test2PasswdEntry.Uid)
		assert.Equal(t, test2HomeDirectory, test2PasswdEntry.HomeDirectory)
		assert.Equal(t, test2StartupCommand, test2PasswdEntry.Shell)
	}

	test2ShadowEntry, err := userutils.GetShadowFileEntryForUser(imageConnection.Chroot().RootDir(), "test2")
	if assert.NoError(t, err) {
		verifyPassword(t, test2ShadowEntry.EncryptedPassword, test2PlainText)

		currentDay := installutils.DaysSinceUnixEpoch()
		assert.Equal(t, currentDay+test2PasswordExpiresDays, int64(*test2ShadowEntry.AccountExpirationDate))
	}

	test2UserGroups, err := userutils.GetUserGroups(imageConnection.Chroot().RootDir(), "test2")
	if assert.NoError(t, err) {
		assert.ElementsMatch(t, test2UserGroups, []string{"sudo"})
	}
}

func TestCustomizeImageUsersExitingUserHomeDir(t *testing.T) {
	baseImage, _ := checkSkipForCustomizeDefaultImage(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageUsersExitingUserHomeDir")
	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	config := imagecustomizerapi.Config{
		OS: &imagecustomizerapi.OS{
			Users: []imagecustomizerapi.User{
				{
					Name:          "root",
					HomeDirectory: "/home/root",
				},
			},
		},
	}

	// Customize image.
	err := CustomizeImage(t.Context(), buildDir, testDir, &config, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.ErrorContains(t, err, "cannot set home directory (/home/root) on a user (root) that already exists")
}

func TestCustomizeImageUsersExitingUserUid(t *testing.T) {
	baseImage, _ := checkSkipForCustomizeDefaultImage(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageUsersExitingUserUid")
	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	config := imagecustomizerapi.Config{
		OS: &imagecustomizerapi.OS{
			Users: []imagecustomizerapi.User{
				{
					Name: "root",
					UID:  ptrutils.PtrTo(1),
				},
			},
		},
	}

	// Customize image.
	err := CustomizeImage(t.Context(), buildDir, testDir, &config, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.ErrorContains(t, err, "cannot set UID (1) on a user (root) that already exists")
}

func TestCustomizeImageUsersMissingSshPublicKeyFile(t *testing.T) {
	baseImage, _ := checkSkipForCustomizeDefaultImage(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageUsersMissingSshPublicKeyFile")
	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	config := imagecustomizerapi.Config{
		OS: &imagecustomizerapi.OS{
			Users: []imagecustomizerapi.User{
				{
					Name: "test",
					SSHPublicKeyPaths: []string{
						"does-not-exist",
					},
				},
			},
		},
	}

	// Customize image.
	err := CustomizeImage(t.Context(), buildDir, testDir, &config, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.ErrorContains(t, err, "failed to find SSH public key file (does-not-exist)")
}

func TestCustomizeImageUsersAddFiles(t *testing.T) {
	baseImage, _ := checkSkipForCustomizeDefaultImage(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageUsersAddFiles")
	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	configFile := filepath.Join(testDir, "add-user-files.yaml")

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	// Connect to image.
	imageConnection, err := connectToCoreEfiImage(buildDir, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	userHomeDir := filepath.Join(imageConnection.Chroot().RootDir(), "/home/test")
	userFilePath := filepath.Join(userHomeDir, "platypus")

	// Ensure user's home directory has correct ownership.
	userHomeDirStat, err := os.Stat(userHomeDir)
	if assert.NoError(t, err) {
		userHomeDirStatSys := userHomeDirStat.Sys().(*syscall.Stat_t)
		assert.Equal(t, uint32(1000), userHomeDirStatSys.Uid)
		assert.Equal(t, uint32(1000), userHomeDirStatSys.Gid)
	}

	// Verity file was copied to image.
	verifyFileContentsEqual(t, userFilePath, "Egg-laying mammal")
}

func verifySshAuthorizedKeys(t *testing.T, rootDir string, homeDirectory string, sshPublicKeys []string,
	gid int, uid int,
) {
	sshDirectory := filepath.Join(rootDir, homeDirectory, userutils.SSHDirectoryName)

	sshDirectoryStat, err := os.Stat(sshDirectory)
	if assert.NoError(t, err) {
		assert.Equal(t, userutils.SshDirectoryPerm, sshDirectoryStat.Mode().Perm())

		sshDirectorySysStat := sshDirectoryStat.Sys().(*syscall.Stat_t)
		assert.Equal(t, uint32(uid), sshDirectorySysStat.Uid)
		assert.Equal(t, uint32(gid), sshDirectorySysStat.Gid)
	}

	authorizedKeysPath := filepath.Join(sshDirectory, userutils.SSHAuthorizedKeysFileName)

	authorizedKeysStat, err := os.Stat(authorizedKeysPath)
	if assert.NoError(t, err) {
		assert.Equal(t, userutils.AuthorizedKeysPerm, authorizedKeysStat.Mode().Perm())

		authorizedKeysSysStat := authorizedKeysStat.Sys().(*syscall.Stat_t)
		assert.Equal(t, uint32(uid), authorizedKeysSysStat.Uid)
		assert.Equal(t, uint32(gid), authorizedKeysSysStat.Gid)
	}

	authorizedKeys, err := file.ReadLines(authorizedKeysPath)
	if assert.NoError(t, err) {
		for _, sshPublicKey := range sshPublicKeys {
			assert.Contains(t, authorizedKeys, sshPublicKey)
		}
	}
}

func verifyPassword(t *testing.T, encryptedPassword string, plainTextPassword string) bool {
	match := shadowPasswordRegexp.FindStringSubmatch(encryptedPassword)
	if !assert.NotNilf(t, match, "parse shadow password field (%s)", encryptedPassword) {
		return false
	}

	id := match[1]

	// 'openssl passwd' allows the number of rounds to be added to the start of the salt arg.
	roundsAndSalt := match[2]

	if !assert.Equal(t, "6", id) {
		return false
	}

	reencryptedPassword, _, err := shell.NewExecBuilder("openssl", "passwd", "-6", "-salt", roundsAndSalt, "-stdin").
		Stdin(plainTextPassword).
		LogLevel(shell.LogDisabledLevel, logrus.DebugLevel).
		ExecuteCaptureOutput()
	if !assert.NoError(t, err) {
		return false
	}

	return assert.Equal(t, encryptedPassword, strings.TrimSpace(reencryptedPassword))
}

func TestCustomizeImageUsersExitingUserPassword(t *testing.T) {
	baseImage, _ := checkSkipForCustomizeDefaultImage(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageUsersExitingUserPassword")

	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	// Create an image with a user that has an initial password.
	passwordValue := "pass"
	configWithPassword := imagecustomizerapi.Config{
		OS: &imagecustomizerapi.OS{
			Users: []imagecustomizerapi.User{
				{
					Name: "testuser",
					Password: &imagecustomizerapi.Password{
						Type:  "plain-text",
						Value: passwordValue,
					},
				},
			},
		},
	}

	err := CustomizeImage(t.Context(), buildDir, testDir, &configWithPassword, baseImage, nil, outImageFilePath, "raw",
		false, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	// Run customization again without providing a password.
	configWithoutPassword := imagecustomizerapi.Config{
		OS: &imagecustomizerapi.OS{
			Users: []imagecustomizerapi.User{
				{
					Name:          "testuser",
					SSHPublicKeys: []string{"ssh-rsa abc123"},
				},
			},
		},
	}

	err = CustomizeImage(t.Context(), buildDir, testDir, &configWithoutPassword, outImageFilePath, nil, outImageFilePath, "raw",
		false, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	// Verify that password was not cleared.
	imageConnection, err := connectToCoreEfiImage(buildDir, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	shadowEntry, err := userutils.GetShadowFileEntryForUser(imageConnection.Chroot().RootDir(), "testuser")
	if assert.NoError(t, err) {
		verifyPassword(t, shadowEntry.EncryptedPassword, passwordValue)
	}
}
