// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The post-A/B ACL layout (azure-container-linux#28): a dedicated dm-verity hash partition is
// inserted after each USR, which moves OEM to 6 and ROOT to 7.
const aclSampleDumpWithHash = `label: gpt
label-id: 7A376709-9E8D-4F83-AF56-C9CFB3FFBE93
device: /dev/loop0
unit: sectors
first-lba: 2048
last-lba: 409566
sector-size: 512

/dev/loop0p1 : start=        2048, size=      393216, type=C12A7328-F81F-11D2-BA4B-00A0C93EC93B, uuid=96EC7BA6-E683-4E06-B402-40F644418B49, name="EFI-SYSTEM"
/dev/loop0p2 : start=      395264, size=     2097152, type=5DFBF5F4-2848-4BAC-AA5E-0D9A20B745A6, uuid=91BCEEAE-BA93-4CAE-BD12-11694ED8B8C5, name="USR-A", attrs="GUID:48,56"
/dev/loop0p3 : start=     2492416, size=       20480, type=77FF5F63-E7B6-4633-ACF4-1565B864C0E6, uuid=B736BAF1-CDB4-4535-BEBA-DDAAA30AD7B7, name="HASH-A"
/dev/loop0p4 : start=     2512896, size=     2097152, type=5DFBF5F4-2848-4BAC-AA5E-0D9A20B745A6, uuid=0241E9F2-CF5A-485C-AEAF-6A1FB19094E8, name="USR-B", attrs="GUID:48,56"
/dev/loop0p5 : start=     4610048, size=       20480, type=77FF5F63-E7B6-4633-ACF4-1565B864C0E6, uuid=35BDF78B-C453-4661-98E6-F834F534EF5B, name="HASH-B"
/dev/loop0p6 : start=     4630528, size=      262144, type=0FC63DAF-8483-4772-8E79-3D69D8477DE4, uuid=2458B054-19AB-4EFD-8738-B04148A3B2CC, name="OEM"
/dev/loop0p7 : start=     4892672, size=    58720256, type=0FC63DAF-8483-4772-8E79-3D69D8477DE4, uuid=17CA28A9-E145-48C6-BC2D-7D7D125804CE, name="ROOT"`

// The proposed hash-signature layout (azure-container-linux#88), which splits each HASH partition
// into HASH and HASH-SIG. IC accepts it ahead of that change landing.
const aclSampleDumpWithHashSig = `label: gpt
label-id: 7A376709-9E8D-4F83-AF56-C9CFB3FFBE93
device: /dev/loop0
unit: sectors
first-lba: 2048
last-lba: 409566
sector-size: 512

/dev/loop0p1 : start=        2048, size=      393216, type=C12A7328-F81F-11D2-BA4B-00A0C93EC93B, uuid=96EC7BA6-E683-4E06-B402-40F644418B49, name="EFI-SYSTEM"
/dev/loop0p2 : start=      395264, size=     2097152, type=5DFBF5F4-2848-4BAC-AA5E-0D9A20B745A6, uuid=91BCEEAE-BA93-4CAE-BD12-11694ED8B8C5, name="USR-A", attrs="GUID:48,56"
/dev/loop0p3 : start=     2492416, size=       18432, type=77FF5F63-E7B6-4633-ACF4-1565B864C0E6, uuid=B736BAF1-CDB4-4535-BEBA-DDAAA30AD7B7, name="HASH-A"
/dev/loop0p4 : start=     2510848, size=        2048, type=C23CE4FF-44BD-4B00-B2D4-B41B3419E02A, uuid=AA11BB22-CC33-4D44-8E55-FF6677889900, name="HASH-SIG-A"
/dev/loop0p5 : start=     2512896, size=     2097152, type=5DFBF5F4-2848-4BAC-AA5E-0D9A20B745A6, uuid=0241E9F2-CF5A-485C-AEAF-6A1FB19094E8, name="USR-B", attrs="GUID:48,56"
/dev/loop0p6 : start=     4610048, size=       18432, type=77FF5F63-E7B6-4633-ACF4-1565B864C0E6, uuid=35BDF78B-C453-4661-98E6-F834F534EF5B, name="HASH-B"
/dev/loop0p7 : start=     4628480, size=        2048, type=C23CE4FF-44BD-4B00-B2D4-B41B3419E02A, uuid=BB22CC33-DD44-4E55-9F66-001122334455, name="HASH-SIG-B"
/dev/loop0p8 : start=     4630528, size=      262144, type=0FC63DAF-8483-4772-8E79-3D69D8477DE4, uuid=2458B054-19AB-4EFD-8738-B04148A3B2CC, name="OEM"
/dev/loop0p9 : start=     4892672, size=    58720256, type=0FC63DAF-8483-4772-8E79-3D69D8477DE4, uuid=17CA28A9-E145-48C6-BC2D-7D7D125804CE, name="ROOT"`

func parseAclDump(t *testing.T, dump string) *aclPartitionTable {
	t.Helper()
	table, err := parseAclPartitionTable(dump)
	require.NoError(t, err)
	return table
}

func TestValidateAclLayoutAcceptsKnownLayouts(t *testing.T) {
	tests := []struct {
		name          string
		dump          string
		partCount     int
		dedicatedHash bool
	}{
		{"legacy inline verity", aclSampleDump, 5, false},
		{"per-slot hash partitions", aclSampleDumpWithHash, 7, true},
		{"hash and hash signature partitions", aclSampleDumpWithHashSig, 9, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := parseAclDump(t, tt.dump)
			require.Len(t, table.partitions, tt.partCount)

			err := validateAclLayout(table)
			assert.NoError(t, err)
			assert.Equal(t, tt.dedicatedHash, aclHasDedicatedUsrHashPartitions(table))
		})
	}
}

// Partition order is deliberately not asserted: ACL has already renumbered its partitions once.
func TestValidateAclLayoutIgnoresOrderAndUnknownPartitions(t *testing.T) {
	table := parseAclDump(t, aclSampleDumpWithHash)

	table.partitions[0], table.partitions[6] = table.partitions[6], table.partitions[0]
	assert.NoError(t, validateAclLayout(table), "partition order must not be asserted")

	unknown := *table.partitions[5]
	unknown.label = "SOME-FUTURE-PARTITION"
	table.partitions = append(table.partitions, &unknown)
	assert.NoError(t, validateAclLayout(table), "unknown partitions must be tolerated")
}

func TestValidateAclLayoutRejectsMissingRequiredPartition(t *testing.T) {
	table := parseAclDump(t, aclSampleDumpWithHash)
	table.partitions = append(table.partitions[:6], table.partitions[6+1:]...) // drop ROOT

	err := validateAclLayout(table)
	assert.ErrorIs(t, err, ErrAclGrowUnexpectedLayout)
	assert.ErrorContains(t, err, aclPartLabelRoot)
}

// A hash partition without its data partition cannot be grown coherently.
func TestValidateAclLayoutRejectsOrphanedHashPartition(t *testing.T) {
	table := parseAclDump(t, aclSampleDumpWithHash)
	table.partitions[1].label = "SOMETHING-ELSE" // USR-A -> not USR-A, leaving HASH-A orphaned

	err := validateAclLayout(table)
	assert.ErrorIs(t, err, ErrAclGrowUnexpectedLayout)
	assert.ErrorContains(t, err, aclPartLabelUsrA)
}

// The sizes IC derives must match the sizes ACL picked by hand for its own layout, or a grown image
// would disagree with a freshly built one.
func TestAclUsrHashPartitionSizeMatchesAclBaseLayout(t *testing.T) {
	const MiB = uint64(1024 * 1024)

	tests := []struct {
		usrSize      uint64
		expectedSize uint64
	}{
		// 1 GiB /usr -> 9 MiB, matching HASH-A in azure-container-linux#88 (18432 512-byte blocks).
		{1024 * MiB, 9 * MiB},
		{2048 * MiB, 17 * MiB},
		{4096 * MiB, 33 * MiB},
	}

	for _, tt := range tests {
		size, err := aclUsrHashPartitionSize(tt.usrSize)
		require.NoError(t, err)
		assert.Equal(t, tt.expectedSize, size,
			"hash partition size for a %s /usr", imagecustomizerapi.DiskSize(tt.usrSize).HumanReadable())
		assert.Zero(t, size%MiB, "hash partition size must be MiB-aligned")
	}
}

func TestResolveAclRequestedSizesGrowsHashPartitionsWithUsr(t *testing.T) {
	table := parseAclDump(t, aclSampleDumpWithHash)

	acl := &imagecustomizerapi.Acl{
		Usr: &imagecustomizerapi.AclPartitionGrow{Size: imagecustomizerapi.DiskSize(2048 * 1024 * 1024)},
	}

	sizes, err := resolveAclRequestedSizes(acl, table)
	require.NoError(t, err)

	assert.Equal(t, uint64(2048*1024*1024), sizes[aclPartLabelUsrA])
	assert.Equal(t, uint64(2048*1024*1024), sizes[aclPartLabelUsrB])
	// Both slots' hash partitions grow to hold the larger hash tree.
	assert.Equal(t, uint64(17*1024*1024), sizes[aclPartLabelHashA])
	assert.Equal(t, uint64(17*1024*1024), sizes[aclPartLabelHashB])
	assert.NotContains(t, sizes, aclPartLabelRoot)
	assert.NotContains(t, sizes, aclPartLabelOem)
}

// Signature partitions hold a signature, not the hash tree, so they do not scale with /usr.
func TestResolveAclRequestedSizesLeavesHashSigPartitionsAlone(t *testing.T) {
	table := parseAclDump(t, aclSampleDumpWithHashSig)

	acl := &imagecustomizerapi.Acl{
		Usr: &imagecustomizerapi.AclPartitionGrow{Size: imagecustomizerapi.DiskSize(2048 * 1024 * 1024)},
	}

	sizes, err := resolveAclRequestedSizes(acl, table)
	require.NoError(t, err)

	assert.Equal(t, uint64(17*1024*1024), sizes[aclPartLabelHashA])
	assert.NotContains(t, sizes, aclPartLabelHashSigA)
	assert.NotContains(t, sizes, aclPartLabelHashSigB)
}

// On the legacy layout there is no hash partition to size, and the inline reservation still applies.
func TestResolveAclRequestedSizesLegacyLayoutHasNoHashPartitions(t *testing.T) {
	table := parseAclDump(t, aclSampleDump)

	acl := &imagecustomizerapi.Acl{
		Usr: &imagecustomizerapi.AclPartitionGrow{Size: imagecustomizerapi.DiskSize(2048 * 1024 * 1024)},
	}

	sizes, err := resolveAclRequestedSizes(acl, table)
	require.NoError(t, err)

	assert.Equal(t, uint64(2048*1024*1024), sizes[aclPartLabelUsrA])
	assert.NotContains(t, sizes, aclPartLabelHashA)
	assert.NotContains(t, sizes, aclPartLabelHashB)
}

// A hash partition that is already larger than required is left alone rather than reported as a
// shrink, because its size is derived by IC rather than requested by the user.
func TestResolveAclRequestedSizesKeepsOversizedHashPartition(t *testing.T) {
	table := parseAclDump(t, aclSampleDumpWithHash)

	for _, p := range table.partitions {
		if p.label == aclPartLabelHashA || p.label == aclPartLabelHashB {
			p.sizeSect = 64 * 1024 * 1024 / table.sectorSize // 64 MiB, far beyond what is needed
		}
	}

	acl := &imagecustomizerapi.Acl{
		Usr: &imagecustomizerapi.AclPartitionGrow{Size: imagecustomizerapi.DiskSize(2048 * 1024 * 1024)},
	}

	sizes, err := resolveAclRequestedSizes(acl, table)
	require.NoError(t, err)

	assert.NotContains(t, sizes, aclPartLabelHashA)
	assert.NotContains(t, sizes, aclPartLabelHashB)
}

// Growing the partitions must keep every partition's identity and leave the ones that are not
// growing at their original size, regardless of how many partitions the layout has.
func TestApplyAclGrownLayoutWithHashPartitions(t *testing.T) {
	table := parseAclDump(t, aclSampleDumpWithHash)

	originalRootSize := table.partitions[6].sizeSect
	originalOemSize := table.partitions[5].sizeSect

	acl := &imagecustomizerapi.Acl{
		Usr: &imagecustomizerapi.AclPartitionGrow{Size: imagecustomizerapi.DiskSize(2048 * 1024 * 1024)},
	}
	sizes, err := resolveAclRequestedSizes(acl, table)
	require.NoError(t, err)

	growth, espRecreated := applyAclGrownLayout(table, sizes)

	assert.False(t, espRecreated)
	assert.Greater(t, growth, uint64(0))
	assert.Equal(t, originalRootSize, table.partitions[6].sizeSect, "ROOT must not be resized")
	assert.Equal(t, originalOemSize, table.partitions[5].sizeSect, "OEM must not be resized")

	// Partitions stay contiguous and in order after repacking.
	for i := 1; i < len(table.partitions); i++ {
		prev := table.partitions[i-1]
		assert.Equal(t, prev.startSect+prev.sizeSect, table.partitions[i].startSect,
			"partition %d must start where partition %d ends", i+1, i)
	}
}
