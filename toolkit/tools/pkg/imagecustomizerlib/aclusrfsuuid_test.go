// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/ptrutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAclUsrUuidRegenerationValidation(t *testing.T) {
	for _, test := range []struct {
		name      string
		acl       *imagecustomizerapi.Acl
		verity    imagecustomizerapi.ReinitializeVerityType
		uki       *imagecustomizerapi.Uki
		expected  bool
		errorText string
	}{
		{name: "omitted"},
		{name: "disabled", acl: &imagecustomizerapi.Acl{RegenerateUsrFsUuid: ptrutils.PtrTo(false)}},
		{name: "needs-reseal", acl: &imagecustomizerapi.Acl{RegenerateUsrFsUuid: ptrutils.PtrTo(true)},
			expected: true, errorText: "storage.reinitializeVerity: all"},
		{name: "needs-uki", acl: &imagecustomizerapi.Acl{RegenerateUsrFsUuid: ptrutils.PtrTo(true)},
			verity: imagecustomizerapi.ReinitializeVerityTypeAll, expected: true, errorText: "os.uki.mode: create"},
		{name: "rejects-passthrough", acl: &imagecustomizerapi.Acl{RegenerateUsrFsUuid: ptrutils.PtrTo(true)},
			verity: imagecustomizerapi.ReinitializeVerityTypeAll,
			uki:    &imagecustomizerapi.Uki{Mode: imagecustomizerapi.UkiModePassthrough}, expected: true,
			errorText: "os.uki.mode: create"},
		{name: "enabled", acl: &imagecustomizerapi.Acl{RegenerateUsrFsUuid: ptrutils.PtrTo(true)},
			verity: imagecustomizerapi.ReinitializeVerityTypeAll,
			uki:    &imagecustomizerapi.Uki{Mode: imagecustomizerapi.UkiModeCreate}, expected: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, aclUsrUuidRegenerationEnabled(test.acl))
			config := &ResolvedConfig{Acl: test.acl, Uki: test.uki}
			config.Storage.ReinitializeVerity = test.verity
			err := (&aclDistroHandler{}).checkForUnsupportedApis(config)
			if test.errorText == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, test.errorText)
			}
		})
	}
}

func TestAclUsrUuidPartition(t *testing.T) {
	for _, test := range []struct {
		name      string
		active    string
		data      string
		hash      string
		fsType    string
		mounted   string
		missing   bool
		duplicate bool
		expected  string
		errorText string
	}{
		{name: "active-a", active: "a", data: "a", hash: "hash", fsType: "btrfs", expected: "a"},
		{name: "active-b", active: "b", data: "b", hash: "hash", fsType: "btrfs", expected: "b"},
		{name: "missing-usr-mount", active: "", data: "a", hash: "hash", fsType: "btrfs",
			errorText: "own partition"},
		{name: "not-btrfs", active: "a", data: "a", hash: "hash", fsType: "ext4",
			errorText: "must use Btrfs"},
		{name: "still-mounted", active: "a", data: "a", hash: "hash", fsType: "btrfs", mounted: "/image/usr",
			errorText: "still mounted"},
		{name: "missing-verity", active: "a", fsType: "btrfs", missing: true,
			errorText: "expected one /usr verity device"},
		{name: "wrong-verity-slot", active: "b", data: "a", hash: "hash", fsType: "btrfs",
			errorText: "does not match"},
		{name: "missing-hash", active: "a", data: "a", hash: "absent", fsType: "btrfs",
			errorText: "hash partition is missing"},
		{name: "duplicate-verity", active: "a", data: "a", hash: "hash", fsType: "btrfs", duplicate: true,
			errorText: "expected one /usr verity device"},
	} {
		t.Run(test.name, func(t *testing.T) {
			partitions := []diskutils.PartitionInfo{
				{PartUuid: "root", FileSystemType: "btrfs"},
				{PartUuid: "a", PartLabel: aclPartLabelUsrA, FileSystemType: test.fsType, Mountpoint: test.mounted},
				{PartUuid: "b", PartLabel: aclPartLabelUsrB, FileSystemType: test.fsType, Mountpoint: test.mounted},
				{PartUuid: "hash", FileSystemType: "DM_verity_hash"},
			}
			layout := []fstabEntryPartNum{{PartUuid: "root", FstabEntry: diskutils.FstabEntry{Target: "/"}}}
			if test.active != "" {
				layout = append(layout, fstabEntryPartNum{
					PartUuid: test.active, FstabEntry: diskutils.FstabEntry{Target: "/usr"}, IsVerity: true,
				})
			}
			verity := []verityDeviceMetadata{{
				name: imagecustomizerapi.VerityUsrDeviceName, dataPartUuid: test.data, hashPartUuid: test.hash,
			}}
			if test.missing {
				verity = nil
			}
			if test.duplicate {
				verity = append(verity, verity[0])
			}
			partition, err := aclUsrUuidPartition(layout, verity, partitions)
			if test.errorText != "" {
				assert.ErrorContains(t, err, test.errorText)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expected, partition.PartUuid)
			}
		})
	}
}
