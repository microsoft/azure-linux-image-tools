// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Utility to encrypt disks and partitions

package diskutils

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/configuration"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
)

const (
	// DefaultKeyFilePath points to the initramfs keyfile for the install chroot
	DefaultKeyFilePath = "/etc/default.keyfile"
)

const (
	mappingEncryptedPrefix = "luks-"
	defaultKeyFileName     = "default.keyfile"
)

// EncryptedRootDevice holds settings for an encrypted root partition or disk
type EncryptedRootDevice struct {
	Device      string
	LuksUUID    string
	HostKeyFile string
}

// IsEncryptedDevice checks if a given device is a luks or LVM encrypted device
// - devicePath is the device to check
func IsEncryptedDevice(devicePath string) (result bool) {
	luksPrefix := filepath.Join(mappingFilePath, mappingEncryptedPrefix)
	if strings.HasPrefix(devicePath, luksPrefix) {
		result = true
		return
	}

	lvmRootPath := GetEncryptedRootVolMapping()
	if strings.HasPrefix(devicePath, lvmRootPath) {
		result = true
		return
	}

	return
}

// encryptRootPartition encrypts the root partition
// - partDevPath is the path of the root partition
// - partition is the configuration
// - encrypt is the root encryption settings
func encryptRootPartition(partDevPath string, partition configuration.Partition, encrypt configuration.RootEncryption) (encryptedRoot EncryptedRootDevice, err error) {
	const (
		defaultCipher  = "aes-xts-plain64"
		defaultKeySize = "256"
		defaultHash    = "sha512"
		defaultLuks    = "luks1"
	)
	if encrypt.Enable == false {
		err = fmt.Errorf("encryption not enabled for partition %v", partition.ID)
		return
	}

	encryptedRoot.Device = partDevPath

	// Encrypt the partition
	cryptsetupArgs := []string{
		"--cipher", defaultCipher,
		"--key-size", defaultKeySize,
		"--hash", defaultHash,
		"--type", defaultLuks,
		"luksFormat", partDevPath,
	}
	_, stderr, err := shell.ExecuteWithStdin(encrypt.Password, "cryptsetup", cryptsetupArgs...)

	if err != nil {
		err = fmt.Errorf("failed to encrypt partition (%v):\n%v\n%w", partDevPath, stderr, err)
		return
	}

	logger.Log.Infof("Encrypted partition %v", partition.ID)

	// Open the partition
	uuid, err := getPartUUID(partDevPath)
	if err != nil || uuid == "" {
		err = fmt.Errorf("failed to get UUID for partition (%v):\n%w", partDevPath, err)
		return
	}

	encryptedRoot.LuksUUID = uuid

	blockDevice := fmt.Sprintf("%v%v", mappingEncryptedPrefix, uuid)

	_, stderr, err = shell.ExecuteWithStdin(encrypt.Password, "cryptsetup", "-q", "open", partDevPath, blockDevice)
	if err != nil {
		err = fmt.Errorf("failed to open encrypted partition (%v):\n%v\n%w", partDevPath, stderr, err)
		return
	}

	// Add the LVM
	fullMappedPath, err := enableLVMForEncryptedRoot(filepath.Join(mappingFilePath, blockDevice))
	if err != nil {
		err = fmt.Errorf("failed to enable LVM for encrypted root:\n%w", err)
		return
	}

	// Create the file system
	_, stderr, err = shell.Execute("mkfs", "-t", partition.FsType, fullMappedPath)
	if err != nil {
		err = fmt.Errorf("failed to mkfs for partition (%v):\n%v\n%w", partDevPath, stderr, err)
	}

	return
}
