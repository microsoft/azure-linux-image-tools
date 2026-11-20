// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package userutils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sshTestChroot struct {
	safechroot.DummyChroot
	rootDir string
}

func (c *sshTestChroot) RootDir() string {
	return c.rootDir
}

func TestProvisionUserSSHCerts(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test must be run as root to verify UID/GID ownership")
	}

	const (
		uid = 1234
		gid = 2345
	)

	catPath, err := exec.LookPath("cat")
	require.NoError(t, err)

	binDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "id"), []byte(
		"#!/bin/sh\ncase \"$1:$2\" in\n-u:test) echo 1234;;\n-g:test) echo 2345;;\n*) exit 1;;\nesac\n"), 0o755))
	t.Setenv("PATH", binDir)

	for _, test := range []struct {
		name           string
		homeDir        string
		newParents     []string
		existingParent bool
		existingHome   bool
		existingSSH    bool
	}{
		{name: "missing_home", homeDir: "/home/test"},
		{
			name:       "missing_parent",
			homeDir:    "/home/team/test",
			newParents: []string{"home/team"},
		},
		{
			name:       "missing_parents",
			homeDir:    "/home/team/project/test",
			newParents: []string{"home/team", "home/team/project"},
		},
		{name: "existing_parent", homeDir: "/home/team/test", existingParent: true},
		{name: "existing_home", homeDir: "/home/test", existingHome: true},
		{name: "existing_ssh", homeDir: "/home/test", existingHome: true, existingSSH: true},
	} {
		for _, umask := range []int{0o022, 0o027, 0o077, 0o777} {
			t.Run(fmt.Sprintf("%s/umask_%03o", test.name, umask), func(t *testing.T) {
				rootDir := t.TempDir()
				require.NoError(t, os.Chmod(filepath.Dir(rootDir), 0o755))
				require.NoError(t, os.Chmod(rootDir, 0o755))
				require.NoError(t, os.MkdirAll(filepath.Join(rootDir, "etc"), 0o755))
				require.NoError(t, os.Mkdir(filepath.Join(rootDir, "home"), 0o755))
				require.NoError(t, os.Chmod(filepath.Join(rootDir, "home"), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(rootDir, PasswdFile),
					[]byte("test:x:1234:2345::"+test.homeDir+":/bin/sh\n"), 0o644))

				homePath := filepath.Join(rootDir, test.homeDir)
				parentPath := filepath.Dir(homePath)
				sshPath := filepath.Join(homePath, SSHDirectoryName)
				keysPath := filepath.Join(sshPath, SSHAuthorizedKeysFileName)
				homeMode, homeUID, homeGID := os.FileMode(0o700), uid, gid
				sshMode := SshDirectoryPerm
				expectedKeys := "ssh-ed25519 file-key\nssh-ed25519 inline-key\n"
				if test.existingParent {
					require.NoError(t, os.Mkdir(parentPath, 0o711))
					require.NoError(t, os.Chmod(parentPath, 0o711))
					require.NoError(t, os.Chown(parentPath, 3456, 4567))
				}
				if test.existingHome {
					homeMode, homeUID, homeGID = 0o750, 3456, 4567
					require.NoError(t, os.Mkdir(homePath, homeMode))
					require.NoError(t, os.Chmod(homePath, homeMode))
					require.NoError(t, os.Chown(homePath, homeUID, homeGID))
				}
				if test.existingSSH {
					sshMode = 0o750
					require.NoError(t, os.Mkdir(sshPath, sshMode))
					require.NoError(t, os.Chmod(sshPath, sshMode))
					require.NoError(t, os.Chown(sshPath, uid, gid))
					require.NoError(t, os.WriteFile(keysPath, []byte("ssh-ed25519 existing-key\n"), 0o644))
					expectedKeys = "ssh-ed25519 existing-key\n" + expectedKeys
				}
				pubKeyPath := filepath.Join(rootDir, "key.pub")
				require.NoError(t, os.WriteFile(pubKeyPath, []byte("ssh-ed25519 file-key\n"), 0o644))

				oldUmask := syscall.Umask(umask)
				defer syscall.Umask(oldUmask)

				chroot := &sshTestChroot{rootDir: rootDir}
				err := ProvisionUserSSHCerts(chroot, "test", []string{pubKeyPath},
					[]string{"ssh-ed25519 inline-key"}, true)
				require.NoError(t, err)
				if test.existingParent {
					verifySSHPath(t, parentPath, 0o711, 3456, 4567)
				}
				for _, parent := range test.newParents {
					verifySSHPath(t, filepath.Join(rootDir, parent), 0o755, os.Geteuid(), os.Getegid())
				}
				verifySSHPath(t, homePath, homeMode, homeUID, homeGID)
				verifySSHPath(t, sshPath, sshMode, uid, gid)
				verifySSHPath(t, keysPath, AuthorizedKeysPerm, uid, gid)
				user := &syscall.Credential{
					Uid: uid, Gid: gid, Groups: []uint32{uint32(homeGID)},
				}
				assert.Equal(t, expectedKeys, readSSHKeysAsUser(t, catPath, keysPath, user))

				for range 2 {
					err = ProvisionUserSSHCerts(chroot, "test", nil, []string{"ssh-ed25519 replacement-key"}, false)
					require.NoError(t, err)
					verifySSHPath(t, homePath, homeMode, homeUID, homeGID)
					verifySSHPath(t, sshPath, sshMode, uid, gid)
					verifySSHPath(t, keysPath, AuthorizedKeysPerm, uid, gid)
					assert.Equal(t, "ssh-ed25519 replacement-key\n", readSSHKeysAsUser(t, catPath, keysPath, user))
				}
			})
		}
	}
}

func readSSHKeysAsUser(t *testing.T, catPath, keysPath string, user *syscall.Credential) string {
	t.Helper()
	cmd := exec.Command(catPath, keysPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: user}
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "target user cannot read authorized_keys: %s", output)
	return string(output)
}

func verifySSHPath(t *testing.T, path string, mode os.FileMode, uid, gid int) {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, mode, info.Mode().Perm(), path)
	stat := info.Sys().(*syscall.Stat_t)
	assert.Equal(t, uint32(uid), stat.Uid, path)
	assert.Equal(t, uint32(gid), stat.Gid, path)
}

func TestProvisionUserSSHCertsWithoutKeys(t *testing.T) {
	rootDir := t.TempDir()
	err := ProvisionUserSSHCerts(safechroot.NewChroot(rootDir, true), "test", nil, nil, true)
	require.NoError(t, err)
	entries, err := os.ReadDir(rootDir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestUserSSHDirectory(t *testing.T) {
	expected := "/home/1001/.ssh"
	actual, err := UserSSHDirectory(testDataDir, "test")
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestNameIsValidRoot(t *testing.T) {
	err := NameIsValid("root")
	assert.NoError(t, err)
}

func TestNameIsValidEmpty(t *testing.T) {
	err := NameIsValid("   ")
	assert.ErrorContains(t, err, "invalid")
	assert.ErrorContains(t, err, "name")
}

func TestUIDIsValidRoot(t *testing.T) {
	err := UIDIsValid(0)
	assert.NoError(t, err)
}

func TestUIDIsValidNegative(t *testing.T) {
	err := UIDIsValid(-1)
	assert.ErrorContains(t, err, "invalid")
	assert.ErrorContains(t, err, "UID")
}

func TestUIDIsValidTooLarge(t *testing.T) {
	err := UIDIsValid(60001)
	assert.ErrorContains(t, err, "invalid")
	assert.ErrorContains(t, err, "UID")
}

func TestPasswordExpiresDaysIsValidNoExpiry(t *testing.T) {
	err := PasswordExpiresDaysIsValid(-1)
	assert.NoError(t, err)
}

func TestPasswordExpiresDaysIsValidNegative(t *testing.T) {
	err := PasswordExpiresDaysIsValid(-2)
	assert.ErrorContains(t, err, "invalid")
	assert.ErrorContains(t, err, "PasswordExpiresDays")
}

func TestPasswordExpiresDaysIsValidTooLarge(t *testing.T) {
	err := PasswordExpiresDaysIsValid(100000)
	assert.ErrorContains(t, err, "invalid")
	assert.ErrorContains(t, err, "PasswordExpiresDays")
}

func TestHashPasswordEmpty(t *testing.T) {
	hashedPassword, err := HashPassword("")
	assert.NoError(t, err, "hash password")
	assert.Equal(t, "", hashedPassword)
}

func TestHashPasswordNotEmpty(t *testing.T) {
	hashedPassword, err := HashPassword("password")
	assert.NoError(t, err, "hash password")
	assert.True(t, strings.HasPrefix(hashedPassword, "$6$"), "password prefix")
}

func TestUpdateUserPasswordEmptyToEmpty(t *testing.T) {
	testUpdateUserPassword(t, "root:*:19634:7:99999:7:::", "root:*:19634:7:99999:7:::", "root", "")
}

func TestUpdateUserPasswordSomethingToEmpty(t *testing.T) {
	testUpdateUserPassword(t,
		"root:$6$E0M9VkDvOLvO$nr9FjmIiSSP5C5V3Lhuqv4VzWmscABoiQ0mF.ZTbwKEN4nS60nsiU17qA/RGMbXHtJfci/DeLT1Zu2nhNFbwQ.:19634:7:99999:7:::",
		"root:*:19634:7:99999:7:::",
		"root",
		"")
}

func TestUpdateUserPassword(t *testing.T) {
	testUpdateUserPassword(t,
		"root:*:19634:7:99999:7:::",
		"root:$6$E0M9VkDvOLvO$nr9FjmIiSSP5C5V3Lhuqv4VzWmscABoiQ0mF.ZTbwKEN4nS60nsiU17qA/RGMbXHtJfci/DeLT1Zu2nhNFbwQ.:19634:7:99999:7:::",
		"root",
		"$6$E0M9VkDvOLvO$nr9FjmIiSSP5C5V3Lhuqv4VzWmscABoiQ0mF.ZTbwKEN4nS60nsiU17qA/RGMbXHtJfci/DeLT1Zu2nhNFbwQ.")
}

func testUpdateUserPassword(t *testing.T, originalShadowFile string, expectedShadowFile string, user string, hashedPassword string) {
	rootFilePath := tmpDir

	writeTestShadowFile(t, rootFilePath, originalShadowFile)

	err := UpdateUserPassword(rootFilePath, user, hashedPassword)
	if !assert.NoError(t, err, "update password") {
		return
	}

	actualShadowFileBytes, err := os.ReadFile(filepath.Join(rootFilePath, ShadowFile))
	if !assert.NoError(t, err, "read updated shadow file") {
		return
	}

	assert.Equal(t, expectedShadowFile, string(actualShadowFileBytes))
}

func TestUpdateUserPasswordMissingUser(t *testing.T) {
	rootFilePath := tmpDir

	writeTestShadowFile(t, rootFilePath, "root:!:19634:7:99999:7:::")

	err := UpdateUserPassword(rootFilePath, "test", "")
	if !assert.ErrorContains(t, err, "failed to find user", "update password") {
		return
	}
}

func writeTestShadowFile(t *testing.T, rootFilePath string, content string) {
	shadowFilePath := filepath.Join(rootFilePath, ShadowFile)

	err := os.MkdirAll(filepath.Join(rootFilePath, "/etc"), os.ModePerm)
	if !assert.NoError(t, err, "make /etc dir") {
		return
	}

	err = os.WriteFile(shadowFilePath, []byte(content), os.ModePerm)
	if !assert.NoError(t, err, "write sample shadow file") {
		return
	}
}
