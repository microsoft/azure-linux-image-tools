// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
)

const (
	UuidSize uint32 = 16
)

// Create the uuid and return byte array and string representation
func createUuid() ([UuidSize]byte, string, error) {
	uuid, err := generateRandom128BitNumber()
	if err != nil {
		return uuid, "", err
	}
	uuidStr := convertUuidToString(uuid)
	logger.Log.Infof("Image UUID: %s", uuidStr)

	return uuid, uuidStr, nil
}

// Generates a random 128-bit number
func generateRandom128BitNumber() ([UuidSize]byte, error) {
	var randomBytes [UuidSize]byte
	_, err := rand.Read(randomBytes[:])
	if err != nil {
		return randomBytes, fmt.Errorf("failed to generate random 128-bit number for uuid:\n%w", err)
	}
	return randomBytes, nil
}

func convertUuidToString(uuid [UuidSize]byte) string {
	uuidStr := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4],
		uuid[4:6],
		uuid[6:8],
		uuid[8:10],
		uuid[10:16],
	)

	return uuidStr
}

func extractImageUUID(imageConnection *ImageConnection) ([UuidSize]byte, string, error) {
	var emptyUuid [UuidSize]byte

	releasePath := filepath.Join(imageConnection.Chroot().RootDir(), "etc/image-customizer-release")
	data, err := file.Read(releasePath)
	if err != nil {
		return emptyUuid, "", fmt.Errorf("failed to read %s:\n%w", releasePath, err)
	}

	lines := strings.Split(string(data), "\n")
	var uuidStr string
	for _, line := range lines {
		if strings.HasPrefix(line, "IMAGE_UUID=") {
			uuidStr = strings.Trim(strings.TrimPrefix(line, "IMAGE_UUID="), `"`)
			break
		}
	}

	if uuidStr == "" {
		return emptyUuid, "", fmt.Errorf("IMAGE_UUID not found in %s", releasePath)
	}

	parsed, err := parseUuidString(uuidStr)
	if err != nil {
		return emptyUuid, "", fmt.Errorf("failed to parse IMAGE_UUID from string %s:\n%w", uuidStr, err)
	}

	return parsed, uuidStr, nil
}

func parseUuidString(s string) ([UuidSize]byte, error) {
	var uuid [UuidSize]byte
	n, err := fmt.Sscanf(s,
		"%08x-%04x-%04x-%04x-%012x",
		new(uint32),
		new(uint16),
		new(uint16),
		new(uint16),
		new(uint64),
	)
	if err != nil || n != 5 {
		return uuid, fmt.Errorf("invalid UUID format")
	}

	// Parse directly into a byte array for safety
	parts := strings.Split(s, "-")
	if len(parts) != 5 {
		return uuid, fmt.Errorf("invalid UUID format")
	}
	raw := strings.Join(parts, "")
	bytes, err := hex.DecodeString(raw)
	if err != nil || len(bytes) != 16 {
		return uuid, fmt.Errorf("failed to decode UUID")
	}
	copy(uuid[:], bytes)
	return uuid, nil
}
