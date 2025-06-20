// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package randomization

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
)

const (
	UuidSize uint32 = 16
)

// Create the uuid and return byte array and string representation
func CreateUuid() ([UuidSize]byte, string, error) {
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

func ParseUuidString(s string) ([UuidSize]byte, error) {
	var uuid [UuidSize]byte

	parts := strings.Split(s, "-")
	if len(parts) != 5 {
		return uuid, fmt.Errorf("invalid UUID format: expected 5 parts, got (%d)", len(parts))
	}
	raw := strings.Join(parts, "")
	bytes, err := hex.DecodeString(raw)
	if err != nil {
		return uuid, fmt.Errorf("failed to decode UUID hex string:\n%w", err)
	}
	if len(bytes) != 16 {
		return uuid, fmt.Errorf("decoded UUID has incorrect length: expected 16, got (%d)", len(bytes))
	}

	copy(uuid[:], bytes)
	return uuid, nil
}
