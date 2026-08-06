// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// This test guards the COSI metadata that Image Customizer emits against the
// JSON Schema published by Trident (the downstream consumer). If a change to
// the cosiapi types drifts the emitted metadata.json away from the contract
// Trident expects, this test fails in IC's own CI, so the break is caught here
// rather than downstream when Trident picks up a new IC build.
//
// The schema is fetched live from the Trident repository at test time so the
// test always validates against the current contract (rather than a vendored
// snapshot that can silently go stale). This means the test requires network
// access to raw.githubusercontent.com.

package cosiapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cosiSchemaURL is the canonical location of the COSI metadata JSON Schema,
// authored and maintained in the Trident repository (matches the schema's
// own "$id").
const cosiSchemaURL = "https://raw.githubusercontent.com/microsoft/trident/main/docs/Reference/Composable-OS-Image/cosi-metadata-v1.2.schema.json"

// sampleSha384 is a syntactically valid 96-hex-character SHA-384 digest.
var sampleSha384 = strings.Repeat("a", 96)

// fetchCosiSchemaBytes downloads the COSI schema, retrying a few times to ride
// out transient network blips.
func fetchCosiSchemaBytes(t *testing.T) []byte {
	t.Helper()

	client := &http.Client{Timeout: 30 * time.Second}

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		body, err := func() ([]byte, error) {
			resp, err := client.Get(cosiSchemaURL)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("unexpected status %s", resp.Status)
			}
			return io.ReadAll(resp.Body)
		}()
		if err == nil {
			return body
		}

		lastErr = err
		if attempt < 3 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	require.NoError(t, lastErr, "fetching COSI schema from %s", cosiSchemaURL)
	return nil
}

func loadCosiSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()

	schemaBytes := fetchCosiSchemaBytes(t)

	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaBytes))
	require.NoError(t, err, "parsing COSI schema")

	compiler := jsonschema.NewCompiler()
	err = compiler.AddResource(cosiSchemaURL, schemaDoc)
	require.NoError(t, err, "adding COSI schema resource")

	schema, err := compiler.Compile(cosiSchemaURL)
	require.NoError(t, err, "compiling COSI schema")

	return schema
}

func validateAgainstSchema(t *testing.T, schema *jsonschema.Schema, metadata MetadataJson) {
	t.Helper()

	metadataBytes, err := json.Marshal(metadata)
	require.NoError(t, err, "marshaling metadata")

	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(metadataBytes))
	require.NoError(t, err, "re-parsing marshaled metadata")

	err = schema.Validate(instance)
	if err != nil {
		t.Errorf("emitted COSI metadata.json does not satisfy the Trident COSI schema "+
			"(%s). The cosiapi types have drifted from the contract Trident consumes; "+
			"update the types to match the current schema.\nmetadata:\n%s\nvalidation error:\n%v",
			cosiSchemaURL, string(metadataBytes), err)
	}
}

func sampleImageFile(path string) ImageFile {
	return ImageFile{
		Path:             path,
		CompressedSize:   1024,
		UncompressedSize: 4096,
		Sha384:           sampleSha384,
	}
}

// grubImageMetadata is a representative COSI for a GRUB-booted image with a
// dm-verity-sealed root filesystem.
func grubImageMetadata() MetadataJson {
	partition1 := 1
	partition2 := 2
	hashOffset := uint64(536870912)

	return MetadataJson{
		Version:   "1.2",
		OsArch:    "x86_64",
		OsRelease: "NAME=\"Microsoft Azure Linux\"\nVERSION=\"3.0\"\n",
		Id:        "6f3b1c2a-4d5e-6f70-8192-a3b4c5d6e7f8",
		Disk: Disk{
			Size:    4294967296,
			Type:    DiskTypeGpt,
			LbaSize: 512,
			GptRegions: []GptDiskRegion{
				{Image: sampleImageFile("images/gpt.raw.zst"), Type: RegionTypePrimaryGpt},
				{Image: sampleImageFile("images/esp.raw.zst"), Type: RegionTypePartition, Number: &partition1},
				{Image: sampleImageFile("images/root.raw.zst"), Type: RegionTypePartition, Number: &partition2},
			},
		},
		Images: []FileSystem{
			{
				Image:      sampleImageFile("images/esp.raw.zst"),
				MountPoint: "/boot/efi",
				FsType:     "vfat",
				FsUuid:     "C3D4-250D",
				PartType:   "c12a7328-f81f-11d2-ba4b-00a0c93ec93b",
			},
			{
				Image:      sampleImageFile("images/root.raw.zst"),
				MountPoint: "/",
				FsType:     "ext4",
				FsUuid:     "11111111-2222-3333-4444-555555555555",
				PartType:   "0fc63daf-8483-4772-8e79-3d69d8477de4",
				Verity: &VerityConfig{
					Image:      sampleImageFile("images/root-hash.raw.zst"),
					Roothash:   sampleSha384,
					HashOffset: &hashOffset,
				},
			},
		},
		Bootloader: Bootloader{Type: BootloaderTypeGrub},
		OsPackages: []OsPackage{
			{Name: "kernel", Version: "6.6.2", Release: "1.azl3", Arch: "x86_64"},
			{Name: "systemd", Version: "255", Release: "6.azl3", Arch: "x86_64"},
		},
		Compression: Compression{MaxWindowLog: 27},
	}
}

// systemdBootImageMetadata is a representative COSI for a systemd-boot image
// booting a standalone UKI.
func systemdBootImageMetadata() MetadataJson {
	partition1 := 1

	return MetadataJson{
		Version:   "1.2",
		OsArch:    "aarch64",
		OsRelease: "NAME=\"Microsoft Azure Linux\"\nVERSION=\"3.0\"\n",
		Id:        "1a2b3c4d-5e6f-7081-9203-a4b5c6d7e8f9",
		Disk: Disk{
			Size:    2147483648,
			Type:    DiskTypeGpt,
			LbaSize: 4096,
			GptRegions: []GptDiskRegion{
				{Image: sampleImageFile("images/gpt.raw.zst"), Type: RegionTypePrimaryGpt},
				{Image: sampleImageFile("images/esp.raw.zst"), Type: RegionTypePartition, Number: &partition1},
			},
		},
		Images: []FileSystem{
			{
				Image:      sampleImageFile("images/esp.raw.zst"),
				MountPoint: "/boot/efi",
				FsType:     "vfat",
				FsUuid:     "A1B2-C3D4",
				PartType:   "c12a7328-f81f-11d2-ba4b-00a0c93ec93b",
			},
		},
		Bootloader: Bootloader{
			Type: BootloaderTypeSystemdBoot,
			SystemdBoot: &SystemDBoot{
				Entries: []SystemDBootEntry{
					{
						Type:    SystemDBootEntryTypeUKIStandalone,
						Path:    "/EFI/Linux/azl-6.6.2.efi",
						Cmdline: "root=PARTUUID=... rw quiet",
						Kernel:  "6.6.2-1.azl3",
					},
				},
			},
		},
		OsPackages:  []OsPackage{},
		Compression: Compression{MaxWindowLog: 31},
	}
}

// TestMetadataJsonMatchesTridentSchema validates that representative COSI
// metadata produced from the cosiapi types conforms to the Trident-authored
// COSI JSON Schema. Requires network access (fetches the schema from Trident).
func TestMetadataJsonMatchesTridentSchema(t *testing.T) {
	schema := loadCosiSchema(t)

	testCases := map[string]MetadataJson{
		"grub-with-verity": grubImageMetadata(),
		"systemd-boot-uki": systemdBootImageMetadata(),
	}

	for name, metadata := range testCases {
		t.Run(name, func(t *testing.T) {
			validateAgainstSchema(t, schema, metadata)
		})
	}
}

// TestCosiSchemaTargetsExpectedVersion is a tripwire: if Trident bumps the COSI
// revision, the version constant emitted by IC (cosicommon.go sets
// MetadataJson.Version) must be updated to match.
func TestCosiSchemaTargetsExpectedVersion(t *testing.T) {
	schema := loadCosiSchema(t)

	// A metadata document declaring the version IC currently emits must validate.
	metadata := grubImageMetadata()
	metadata.Version = "1.2"
	validateAgainstSchema(t, schema, metadata)

	// A metadata document declaring a different version must NOT validate,
	// confirming the schema still pins version 1.2.
	metadata.Version = "9.9"
	metadataBytes, err := json.Marshal(metadata)
	require.NoError(t, err)
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(metadataBytes))
	require.NoError(t, err)
	assert.Error(t, schema.Validate(instance),
		"schema should pin metadata version 1.2; a 9.9 document unexpectedly validated")
}
