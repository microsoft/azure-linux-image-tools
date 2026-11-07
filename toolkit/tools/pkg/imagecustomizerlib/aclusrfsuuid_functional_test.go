// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/grub"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/ptrutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestCustomizeImageAclRegenerateUsrFsUuid(t *testing.T) {
	baseImage := checkSkipForCustomizeImage(t, testBaseImageAclCore)
	testTempDir := filepath.Join(tmpDir, t.Name())
	require.NoError(t, os.MkdirAll(testTempDir, 0o755))
	defer os.RemoveAll(testTempDir)

	// Discovery can mount the ESP, so inspect an owned copy, never the supplied base.
	baseRaw := filepath.Join(testTempDir, "base.raw")
	_, err := aclUsrUuidCommand("qemu-img", "convert", "-O", "raw", baseImage, baseRaw)
	require.NoError(t, err)
	base := inspectAclUsrUuidImage(t, baseRaw, filepath.Join(testTempDir, "inspect-base"))
	states := []aclUsrUuidImageState{base}
	images := []string{baseImage}
	seenUuids := map[string]bool{base.usrUuid: true}

	for _, step := range []struct {
		name       string
		input      int
		acl        *imagecustomizerapi.Acl
		regenerate bool
	}{
		{
			name: "enabled-first", input: 0, regenerate: true,
			acl: &imagecustomizerapi.Acl{RegenerateUsrFsUuid: ptrutils.PtrTo(true)},
		},
		{
			name: "enabled-again", input: 1, regenerate: true,
			acl: &imagecustomizerapi.Acl{RegenerateUsrFsUuid: ptrutils.PtrTo(true)},
		},
		{
			name: "explicit-false", input: 2,
			acl: &imagecustomizerapi.Acl{RegenerateUsrFsUuid: ptrutils.PtrTo(false)},
		},
		{name: "omitted", input: 2},
	} {
		passed := t.Run(step.name, func(t *testing.T) {
			stepDir := filepath.Join(testTempDir, step.name)
			buildDir := filepath.Join(stepDir, "build")
			output := filepath.Join(stepDir, "image.raw")
			config := &imagecustomizerapi.Config{
				Acl: step.acl,
				Storage: imagecustomizerapi.Storage{
					ReinitializeVerity: imagecustomizerapi.ReinitializeVerityTypeAll,
				},
				OS: &imagecustomizerapi.OS{
					Uki: &imagecustomizerapi.Uki{Mode: imagecustomizerapi.UkiModeCreate},
					Packages: imagecustomizerapi.Packages{
						Manifest: &imagecustomizerapi.PackageManifest{
							Mode: imagecustomizerapi.PackageManifestModePassthrough,
						},
					},
				},
			}
			previewFeatures := []imagecustomizerapi.PreviewFeature{
				imagecustomizerapi.PreviewFeatureDistroVersion,
				imagecustomizerapi.PreviewFeatureReinitializeVerity,
				imagecustomizerapi.PreviewFeatureUki,
				imagecustomizerapi.PreviewFeaturePackageManifest,
			}
			if step.acl != nil {
				previewFeatures = append(previewFeatures, imagecustomizerapi.PreviewFeatureAclRegenerateUsrFsUuid)
			}
			config.PreviewFeatures = previewFeatures

			require.NoError(t, basicCustomizeImage(t.Context(), buildDir, stepDir, config,
				images[step.input], output, "raw", previewFeatures))
			state := inspectAclUsrUuidImage(t, output, filepath.Join(stepDir, "inspect"))
			input := states[step.input]

			assert.Equal(t, base.gptDump, state.gptDump, "all GPT identities, geometry and attributes must be preserved")
			assert.Equal(t, base.dataPartUuid, state.dataPartUuid, "active /usr partition must not change")
			assert.Equal(t, base.hashPartUuid, state.hashPartUuid, "active hash partition must not change")
			assert.Equal(t, base.inactiveDigests, state.inactiveDigests, "inactive slot bytes must be preserved")
			assert.Equal(t, base.otherFilesystems, state.otherFilesystems, "other filesystem types and UUIDs must be preserved")
			if step.regenerate {
				assert.False(t, seenUuids[state.usrUuid], "each enabled customization must produce a fresh /usr UUID")
				assert.NotEqual(t, input.rootHash, state.rootHash, "changing /usr UUID must change its verity root hash")
				seenUuids[state.usrUuid] = true
			} else {
				assert.Equal(t, input.usrUuid, state.usrUuid, "disabled or omitted opt-in must preserve /usr UUID")
			}

			images = append(images, output)
			states = append(states, state)
		})
		require.True(t, passed, "cannot continue customization chain after %s fails", step.name)
	}
}

type aclUsrUuidImageState struct {
	gptDump          string
	dataPartUuid     string
	hashPartUuid     string
	usrUuid          string
	rootHash         string
	otherFilesystems map[string][2]string
	inactiveDigests  map[string][]byte
}

func inspectAclUsrUuidImage(t *testing.T, imageFile, buildDir string) aclUsrUuidImageState {
	t.Helper()
	require.NoError(t, os.MkdirAll(buildDir, 0o755))
	connection, _, _, err := connectToExistingImage(t.Context(), imageFile, buildDir, filepath.Join(buildDir, "root"),
		false /*includeDefaultMounts*/, true /*readonly*/, true /*readOnlyVerity*/, false /*ignoreOverlays*/, nil)
	require.NoError(t, err)
	defer connection.Close()

	var usrMetadata []verityDeviceMetadata
	for _, metadata := range connection.verityMetadata {
		if metadata.name == imagecustomizerapi.VerityUsrDeviceName {
			usrMetadata = append(usrMetadata, metadata)
		}
	}
	require.Len(t, usrMetadata, 1, "fixture must have exactly one active /usr verity device")
	metadata := usrMetadata[0]
	require.NoError(t, connection.DisconnectChroot())
	require.Nil(t, connection.Chroot(), "discovery mounts must be disconnected before opening the verified consumer")
	partitions, err := diskutils.GetDiskPartitions(connection.Loopback().DevicePath())
	require.NoError(t, err)
	data, _, err := findPartition(imagecustomizerapi.MountIdentifierTypePartUuid, metadata.dataPartUuid, partitions)
	require.NoError(t, err)
	hash, _, err := findPartition(imagecustomizerapi.MountIdentifierTypePartUuid, metadata.hashPartUuid, partitions)
	require.NoError(t, err)
	activeUsr, relativePath, err := getPartitionOfPath("/usr", partitions, connection.partitionsLayout)
	require.NoError(t, err)
	require.Equal(t, ".", relativePath, "fixture requires a dedicated /usr filesystem")
	require.Equal(t, data.PartUuid, activeUsr.PartUuid, "UKI must reference the /usr selected by the discovered layout")
	require.Equal(t, "btrfs", data.FileSystemType, "fixture requires Btrfs /usr")
	require.Empty(t, data.Mountpoint, "raw /usr must be unmounted before opening its verified mapping")
	require.NotEmpty(t, metadata.rootHash)
	if data.PartUuid == hash.PartUuid {
		require.NotZero(t, metadata.formatSettings.hashOffsetBytes, "inline verity requires its actual hash offset")
	}

	usrMountOptions := ""
	for _, entry := range connection.partitionsLayout {
		if entry.FstabEntry.Target == "/usr" {
			usrMountOptions = entry.FstabEntry.FsOptions
		}
	}
	dev := connection.Loopback().DevicePath()
	dump, err := aclUsrUuidCommand("sfdisk", "--dump", dev)
	require.NoError(t, err)
	state := aclUsrUuidImageState{
		// Only the transient loop device name is normalized; retain the complete GPT dump.
		gptDump:          strings.ReplaceAll(dump, dev, "<disk>"),
		dataPartUuid:     data.PartUuid,
		hashPartUuid:     hash.PartUuid,
		rootHash:         metadata.rootHash,
		otherFilesystems: map[string][2]string{},
		inactiveDigests:  map[string][]byte{},
	}
	for _, partition := range partitions {
		// Verity superblock UUIDs are not ordinary filesystem UUIDs and can change on reseal.
		if partition.FileSystemType == "" || strings.EqualFold(partition.FileSystemType, "DM_verity_hash") {
			continue
		}
		fsUuid, err := aclUsrUuidCommand("blkid", "--probe", "--match-tag", "UUID", "--output", "value",
			partition.Path)
		require.NoError(t, err, "probing filesystem UUID for PARTUUID=%s", partition.PartUuid)
		fsUuid = strings.TrimSpace(fsUuid)
		require.NotEmpty(t, fsUuid, "filesystem UUID missing for PARTUUID=%s", partition.PartUuid)
		if partition.PartUuid == data.PartUuid {
			state.usrUuid = fsUuid
		} else {
			state.otherFilesystems[partition.PartUuid] = [2]string{partition.FileSystemType, fsUuid}
		}
	}
	require.NotEmpty(t, state.usrUuid)

	peerUsrLabel, peerHashLabel := "", ""
	switch data.PartLabel {
	case aclPartLabelUsrA:
		peerUsrLabel, peerHashLabel = aclPartLabelUsrB, aclPartLabelHashB
	case aclPartLabelUsrB:
		peerUsrLabel, peerHashLabel = aclPartLabelUsrA, aclPartLabelHashA
	default:
		t.Fatalf("unsupported ACL fixture: active /usr partition label %q is not an A/B slot", data.PartLabel)
	}
	byLabel := partitionsByLabel(partitions)
	peerUsr, ok := byLabel[peerUsrLabel]
	require.True(t, ok, "unsupported ACL fixture: inactive %s partition is absent", peerUsrLabel)
	require.NotEqual(t, hash.PartUuid, peerUsr.PartUuid, "active verity must not use inactive /usr as its hash device")
	state.inactiveDigests[peerUsr.PartUuid] = hashPartitionDigest(t, peerUsr.Path)
	if peerHash, ok := byLabel[peerHashLabel]; ok {
		require.NotEqual(t, hash.PartUuid, peerHash.PartUuid, "active verity must not use the inactive hash slot")
		state.inactiveDigests[peerHash.PartUuid] = hashPartitionDigest(t, peerHash.Path)
	} else {
		require.Equal(t, data.PartUuid, hash.PartUuid,
			"unsupported ACL fixture: dedicated active hash device without a peer hash partition")
		t.Logf("%s is absent in this inline-verity fixture; the inactive %s digest covers its entire partition",
			peerHashLabel, peerUsrLabel)
	}

	esp, err := findSystemBootPartition(partitions)
	require.NoError(t, err)
	espMount, err := safemount.NewMount(esp.Path, filepath.Join(buildDir, "esp"), esp.FileSystemType,
		unix.MS_RDONLY, "", true)
	require.NoError(t, err)
	defer espMount.Close()
	cmdlines, err := extractKernelCmdlineFromUkiEfis(espMount.Target(), buildDir)
	require.NoError(t, err)
	require.NotEmpty(t, cmdlines, "fixture must contain a UKI")
	for kernel, cmdline := range cmdlines {
		tokens, err := grub.TokenizeConfig(cmdline)
		require.NoError(t, err, "UKI %s", kernel)
		args, err := ParseCommandLineArgs(tokens)
		require.NoError(t, err, "UKI %s", kernel)
		_, _, ukiMetadata, err := findVerityPartitionsFromCmdline(partitions, args, "systemd.verity_usr_data",
			"systemd.verity_usr_hash", "usrhash", "systemd.verity_usr_options", imagecustomizerapi.VerityUsrDeviceName)
		require.NoError(t, err, "UKI %s must have resolvable data/hash targets and a readable verity superblock", kernel)
		require.Equal(t, metadata, ukiMetadata, "effective UKI %s /usr verity arguments must match discovered metadata", kernel)
	}
	require.NoError(t, espMount.CleanClose())

	// Discovery alone does not open dm-verity or authenticate filesystem reads.
	verifyAclUsrUuidConsumer(t, buildDir, data.Path, hash.Path, metadata, usrMountOptions)
	require.NoError(t, connection.CleanClose())
	return state
}

func verifyAclUsrUuidConsumer(t *testing.T, buildDir, dataPath, hashPath string,
	metadata verityDeviceMetadata, mountOptions string,
) {
	t.Helper()
	hashOffset := fmt.Sprintf("--hash-offset=%d", metadata.formatSettings.hashOffsetBytes)
	_, err := aclUsrUuidCommand("veritysetup", "verify", hashOffset, dataPath, hashPath, metadata.rootHash)
	require.NoError(t, err, "verify the complete /usr tree against the UKI root hash")

	// Discovery mounted the raw Btrfs device; retire that alias before mounting the same FSID through dm-verity.
	_, err = aclUsrUuidCommand("btrfs", "device", "scan", "--forget", dataPath)
	require.NoError(t, err)
	mapping := "acl-usr-uuid-" + uuid.NewString()
	_, err = aclUsrUuidCommand("veritysetup", "open", hashOffset, dataPath, mapping, hashPath, metadata.rootHash)
	require.NoError(t, err)
	mappingPath := filepath.Join("/dev/mapper", mapping)
	defer func() {
		_, err := aclUsrUuidCommand("btrfs", "device", "scan", "--forget", mappingPath)
		assert.NoError(t, err)
		_, err = aclUsrUuidCommand("veritysetup", "close", "--deferred", mapping)
		assert.NoError(t, err)
	}()

	readOnly, err := aclUsrUuidCommand("blockdev", "--getro", mappingPath)
	require.NoError(t, err)
	require.Equal(t, "1", strings.TrimSpace(readOnly), "verity mapping must be read-only")
	mount, err := safemount.NewMount(mappingPath, filepath.Join(buildDir, "verified-usr"),
		"btrfs", unix.MS_RDONLY, mountOptions, true)
	require.NoError(t, err, "mount /usr through the verified read-only mapping (flags=%#x, options=%q)",
		unix.MS_RDONLY, mountOptions)
	defer func() {
		if !assert.NoError(t, mount.CleanClose()) {
			mount.Close()
		}
	}()

	var stat unix.Statfs_t
	require.NoError(t, unix.Statfs(mount.Target(), &stat))
	assert.NotZero(t, stat.Flags&unix.ST_RDONLY, "consumer mount must be read-only")
	osRelease, err := os.ReadFile(filepath.Join(mount.Target(), "lib", "os-release"))
	require.NoError(t, err, "read /usr/lib/os-release through dm-verity")
	require.Contains(t, string(osRelease), "ID=", "mapped /usr must contain OS identification")
}

func aclUsrUuidCommand(command string, args ...string) (string, error) {
	stdout, stderr, err := shell.NewExecBuilder(command, args...).ExecuteCaptureOutput()
	if err != nil {
		return stdout, fmt.Errorf("command %q failed:\nstdout:\n%s\nstderr:\n%s\n%w",
			append([]string{command}, args...), stdout, stderr, err)
	}
	return stdout, nil
}
