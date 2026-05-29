// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"go.opentelemetry.io/otel"
	"golang.org/x/sys/unix"
)

var (
	ErrConvertNewUuidsDiscovery    = NewImageCustomizerError("ConvertNewUuids:Discovery", "failed to discover image layout for UUID reinitialization")
	ErrConvertNewUuidsReset        = NewImageCustomizerError("ConvertNewUuids:Reset", "failed to reset UUIDs during convert")
	ErrConvertNewUuidsVerityRegen  = NewImageCustomizerError("ConvertNewUuids:VerityRegen", "failed to regenerate verity after UUID reset")
	ErrConvertNewUuidsBootUpdate   = NewImageCustomizerError("ConvertNewUuids:BootUpdate", "failed to update boot config after UUID reset")
	ErrConvertNewUuidsUkiMainSigned = NewImageCustomizerError("ConvertNewUuids:UkiMainSigned", "verity args are embedded in signed main UKI; cannot update without re-signing")
)

// reinitializeUuidsForConvert generates new filesystem and partition UUIDs for a raw image,
// then regenerates verity hashes and updates boot configuration as needed.
//
// Three-phase approach:
//  1. Discovery: connect to image readonly to collect partition layout, verity metadata,
//     distro handler, and UKI addon stub binary path.
//  2. UUID Reset: reset all filesystem and partition UUIDs, fix fstab.
//  3. Verity Regeneration: if verity was present, regenerate hashes and update boot config
//     (GRUB or UKI addons).
func reinitializeUuidsForConvert(ctx context.Context, rawImageFile string, buildDir string) error {
	logger.Log.Infof("Reinitializing UUIDs for convert")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "reinitialize_uuids_for_convert")
	defer span.End()

	// ---- Phase 1: Discovery ----
	// Connect readonly to collect partition layout, verity metadata, and distro info.
	imageConnection, partitionsLayout, verityMetadata, _, err := connectToExistingImage(
		ctx, rawImageFile, buildDir, "imageroot-uuid-discovery",
		true /*includeDefaultMounts*/, true /*readonly*/, true /*readOnlyVerity*/,
		true /*ignoreOverlays*/, nil /*distroHandler — auto-detect*/)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrConvertNewUuidsDiscovery, err)
	}

	// Collect pre-reset partition info for verity mapping.
	origPartitions, err := diskutils.GetDiskPartitions(imageConnection.Loopback().DevicePath())
	if err != nil {
		imageConnection.Close()
		return fmt.Errorf("%w:\n%w", ErrConvertNewUuidsDiscovery, err)
	}

	// Build a mapping from old PartUuid → partition slice index.
	// This survives UUID reset because slice index corresponds to physical position.
	oldPartUuidToIndex := make(map[string]int)
	for i, p := range origPartitions {
		if p.Type == "part" && p.PartUuid != "" {
			oldPartUuidToIndex[p.PartUuid] = i
		}
	}

	hasVerity := len(verityMetadata) > 0

	// Detect distro and UKI presence while the image is mounted.
	var distroHandler DistroHandler
	var isUki bool
	var addonStubHostPath string // path to the addon stub binary copied to buildDir

	if hasVerity {
		distroHandler, err = NewDistroHandlerFromChroot(imageConnection.Chroot())
		if err != nil {
			imageConnection.Close()
			return fmt.Errorf("%w: failed to detect distro:\n%w", ErrConvertNewUuidsDiscovery, err)
		}

		// Detect UKI: try to find UKI files on the ESP.
		isUki, addonStubHostPath, err = detectUkiAndCopyStub(origPartitions, imageConnection, buildDir)
		if err != nil {
			imageConnection.Close()
			return fmt.Errorf("%w: failed to detect UKI:\n%w", ErrConvertNewUuidsDiscovery, err)
		}
	}

	// Done with phase 1. Close the image connection (releases loopback + mounts).
	if err := imageConnection.CleanClose(); err != nil {
		return fmt.Errorf("%w: failed to close discovery connection:\n%w", ErrConvertNewUuidsDiscovery, err)
	}

	// ---- Phase 2: UUID Reset ----
	// Reset all filesystem UUIDs (skipping verity hash partitions) and partition UUIDs, then fix fstab.
	err = resetPartitionsUuidsForConvert(ctx, rawImageFile, buildDir)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrConvertNewUuidsReset, err)
	}

	// ---- Phase 3: Verity Regeneration ----
	if hasVerity {
		err = regenerateVerityForConvert(ctx, rawImageFile, buildDir, origPartitions,
			oldPartUuidToIndex, verityMetadata, partitionsLayout, distroHandler, isUki, addonStubHostPath)
		if err != nil {
			return err
		}
	}

	logger.Log.Infof("UUID reinitialization complete")
	return nil
}

// detectUkiAndCopyStub checks whether the image uses UKI boot and copies the addon stub
// to buildDir so it's available after the chroot is closed.
func detectUkiAndCopyStub(diskPartitions []diskutils.PartitionInfo,
	imageConnection *imageconnection.ImageConnection, buildDir string,
) (bool, string, error) {
	espPartition, err := findSystemBootPartition(diskPartitions)
	if err != nil {
		// No ESP means no UKI.
		return false, "", nil
	}

	tmpDirEsp := filepath.Join(buildDir, "tmp-uuid-esp")
	espMount, err := safemount.NewMount(espPartition.Path, tmpDirEsp, espPartition.FileSystemType,
		unix.MS_RDONLY, "", true /*makeAndDeleteDir*/)
	if err != nil {
		return false, "", fmt.Errorf("failed to mount ESP for UKI detection:\n%w", err)
	}
	defer espMount.Close()

	ukiFiles, err := getUkiFiles(tmpDirEsp)
	if err != nil {
		// glob error is not fatal — means no UKI.
		if closeErr := espMount.CleanClose(); closeErr != nil {
			return false, "", closeErr
		}
		return false, "", nil
	}

	if len(ukiFiles) == 0 {
		if closeErr := espMount.CleanClose(); closeErr != nil {
			return false, "", closeErr
		}
		return false, "", nil
	}

	// UKI detected. Copy the addon stub binary to buildDir from the rootfs.
	_, archConfig, err := getBootArchConfig()
	if err != nil {
		if closeErr := espMount.CleanClose(); closeErr != nil {
			return false, "", closeErr
		}
		return false, "", err
	}

	// Try addon stub, then fall back to EFI stub.
	stubSrcPath := filepath.Join(imageConnection.Chroot().RootDir(), archConfig.ukiAddonStubBinaryPath)
	if _, err := os.Stat(stubSrcPath); err != nil {
		stubSrcPath = filepath.Join(imageConnection.Chroot().RootDir(), archConfig.ukiEfiStubBinaryPath)
		if _, err := os.Stat(stubSrcPath); err != nil {
			if closeErr := espMount.CleanClose(); closeErr != nil {
				return false, "", closeErr
			}
			return false, "", fmt.Errorf("addon stub binary not found at %s or %s",
				archConfig.ukiAddonStubBinaryPath, archConfig.ukiEfiStubBinaryPath)
		}
	}

	// Copy stub to buildDir so it persists after chroot closes.
	stubHostPath := filepath.Join(buildDir, "addon-stub.efi")
	stubContent, err := os.ReadFile(stubSrcPath)
	if err != nil {
		if closeErr := espMount.CleanClose(); closeErr != nil {
			return false, "", closeErr
		}
		return false, "", fmt.Errorf("failed to read addon stub:\n%w", err)
	}
	if err := os.WriteFile(stubHostPath, stubContent, 0o644); err != nil {
		if closeErr := espMount.CleanClose(); closeErr != nil {
			return false, "", closeErr
		}
		return false, "", fmt.Errorf("failed to write addon stub:\n%w", err)
	}

	if closeErr := espMount.CleanClose(); closeErr != nil {
		return false, "", closeErr
	}

	return true, stubHostPath, nil
}

// resetPartitionsUuidsForConvert is like resetPartitionsUuids but skips verity hash partitions
// (since verity will be regenerated in phase 3).
func resetPartitionsUuidsForConvert(ctx context.Context, buildImageFile string, buildDir string) error {
	logger.Log.Infof("Resetting partition UUIDs (convert mode, skipping verity hash)")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "reset_partitions_uuids_convert")
	defer span.End()

	loopback, err := safeloopback.NewLoopback(buildImageFile)
	if err != nil {
		return err
	}
	defer loopback.Close()

	partitions, err := diskutils.GetDiskPartitions(loopback.DevicePath())
	if err != nil {
		return err
	}

	// Reset filesystem UUIDs (skip verity hash partitions).
	newUuids := make([]string, len(partitions))
	for i, partition := range partitions {
		if partition.Type != "part" {
			continue
		}

		newUuid, err := resetFileSystemUuid(partition, true /*skipVerityHash*/)
		if err != nil {
			return fmt.Errorf("%w (partition='%s', type='%s'):\n%w",
				ErrPartitionUuidResetFilesystem, partition.Path, partition.FileSystemType, err)
		}

		newUuids[i] = newUuid
	}

	// Reset PARTUUIDs.
	newPartUuids := make([]string, len(partitions))
	for i, partition := range partitions {
		if partition.Type != "part" {
			continue
		}

		newPartUuid, err := resetPartitionUuid(loopback.DevicePath(), i)
		if err != nil {
			return fmt.Errorf("%w (partition='%s'):\n%w", ErrPartitionUuidUpdate, partition.Path, err)
		}

		newPartUuids[i] = newPartUuid
	}

	// Wait for the partition table updates to be processed.
	err = diskutils.WaitForDiskDevice(loopback.DevicePath())
	if err != nil {
		return err
	}

	// Fix /etc/fstab file.
	err = fixPartitionUuidsInFstabFile(partitions, newUuids, newPartUuids, buildDir)
	if err != nil {
		return err
	}

	err = loopback.CleanClose()
	if err != nil {
		return err
	}

	return nil
}

// regenerateVerityForConvert regenerates verity hashes and updates boot configuration
// after UUIDs have been reset.
func regenerateVerityForConvert(ctx context.Context, rawImageFile string, buildDir string,
	origPartitions []diskutils.PartitionInfo, oldPartUuidToIndex map[string]int,
	verityMetadata []verityDeviceMetadata, partitionsLayout []fstabEntryPartNum,
	distroHandler DistroHandler, isUki bool, addonStubHostPath string,
) error {
	logger.Log.Infof("Regenerating verity after UUID reset")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "regenerate_verity_convert")
	defer span.End()

	loopback, err := safeloopback.NewLoopback(rawImageFile)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrConvertNewUuidsVerityRegen, err)
	}
	defer loopback.Close()

	newPartitions, err := diskutils.GetDiskPartitions(loopback.DevicePath())
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrConvertNewUuidsVerityRegen, err)
	}

	sectorSize, _, err := diskutils.GetSectorSize(loopback.DevicePath())
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrConvertNewUuidsVerityRegen, err)
	}

	// Update verity metadata with new PARTUUIDs and regenerate hashes.
	for i := range verityMetadata {
		metadata := &verityMetadata[i]

		// Map old data PARTUUID → index → new PARTUUID.
		dataIdx, ok := oldPartUuidToIndex[metadata.dataPartUuid]
		if !ok {
			return fmt.Errorf("%w: data partition PARTUUID %s not found in original layout",
				ErrConvertNewUuidsVerityRegen, metadata.dataPartUuid)
		}
		hashIdx, ok := oldPartUuidToIndex[metadata.hashPartUuid]
		if !ok {
			return fmt.Errorf("%w: hash partition PARTUUID %s not found in original layout",
				ErrConvertNewUuidsVerityRegen, metadata.hashPartUuid)
		}

		metadata.dataPartUuid = newPartitions[dataIdx].PartUuid
		metadata.hashPartUuid = newPartitions[hashIdx].PartUuid

		// Regenerate the verity hash tree.
		rootHash, err := verityFormat(loopback.DevicePath(),
			newPartitions[dataIdx].Path, newPartitions[hashIdx].Path,
			false /*shrinkHashPartition*/, sectorSize, metadata.name, metadata.formatSettings)
		if err != nil {
			return fmt.Errorf("%w:\n%w", ErrConvertNewUuidsVerityRegen, err)
		}

		metadata.rootHash = rootHash
	}

	// Refresh partitions after veritysetup format.
	err = diskutils.RefreshPartitions(loopback.DevicePath())
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrConvertNewUuidsVerityRegen, err)
	}

	newPartitions, err = diskutils.GetDiskPartitions(loopback.DevicePath())
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrConvertNewUuidsVerityRegen, err)
	}

	// Update partitionsLayout entries with new PARTUUIDs.
	updatedLayout := updatePartitionsLayoutUuids(partitionsLayout, origPartitions, newPartitions, oldPartUuidToIndex)

	// Update boot configuration.
	if isUki {
		err = updateUkiVerityArgsForConvert(loopback.DevicePath(), newPartitions, verityMetadata,
			updatedLayout, buildDir, addonStubHostPath)
	} else {
		err = updateKernelArgsForVerity(buildDir, newPartitions, verityMetadata,
			false /*isUki*/, updatedLayout, distroHandler)
	}
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrConvertNewUuidsBootUpdate, err)
	}

	err = loopback.CleanClose()
	if err != nil {
		return err
	}

	return nil
}

// updatePartitionsLayoutUuids returns a copy of partitionsLayout with PARTUUIDs updated
// to reflect the new partition table.
func updatePartitionsLayoutUuids(layout []fstabEntryPartNum,
	origPartitions []diskutils.PartitionInfo, newPartitions []diskutils.PartitionInfo,
	oldPartUuidToIndex map[string]int,
) []fstabEntryPartNum {
	updated := make([]fstabEntryPartNum, len(layout))
	copy(updated, layout)

	for i := range updated {
		if updated[i].PartUuid == "" {
			continue
		}
		idx, ok := oldPartUuidToIndex[updated[i].PartUuid]
		if !ok {
			continue
		}
		if idx < len(newPartitions) {
			updated[i].PartUuid = newPartitions[idx].PartUuid
		}
	}

	return updated
}

// updateUkiVerityArgsForConvert updates UKI addon .cmdline sections with new verity args.
// For each UKI file on the ESP, it finds the addon(s), strips old verity args,
// constructs new verity args, and rebuilds the addon using ukify.
func updateUkiVerityArgsForConvert(diskDevicePath string,
	diskPartitions []diskutils.PartitionInfo, verityMetadata []verityDeviceMetadata,
	partitionsLayout []fstabEntryPartNum, buildDir string, addonStubPath string,
) error {
	logger.Log.Infof("Updating UKI verity args for convert")

	// Find the boot/ESP partition.
	bootPartition, _, err := getPartitionOfPath("/boot", diskPartitions, partitionsLayout)
	if err != nil {
		return fmt.Errorf("failed to find /boot partition:\n%w", err)
	}

	newVerityArgs, err := constructVerityKernelCmdlineArgs(verityMetadata, diskPartitions, bootPartition.Uuid)
	if err != nil {
		return fmt.Errorf("failed to construct new verity kernel args:\n%w", err)
	}

	newVerityArgsStr := strings.Join(newVerityArgs, " ")

	// Mount the ESP.
	espPartition, err := findSystemBootPartition(diskPartitions)
	if err != nil {
		return fmt.Errorf("failed to find ESP partition:\n%w", err)
	}

	tmpDirEsp := filepath.Join(buildDir, "tmp-uuid-esp-update")
	espMount, err := safemount.NewMount(espPartition.Path, tmpDirEsp, espPartition.FileSystemType,
		0, "", true /*makeAndDeleteDir*/)
	if err != nil {
		return fmt.Errorf("failed to mount ESP for UKI update:\n%w", err)
	}
	defer espMount.Close()

	ukiFiles, err := getUkiFiles(tmpDirEsp)
	if err != nil {
		return fmt.Errorf("failed to list UKI files:\n%w", err)
	}

	for _, ukiFile := range ukiFiles {
		err := updateUkiFileVerityArgs(ukiFile, buildDir, addonStubPath, newVerityArgsStr)
		if err != nil {
			return fmt.Errorf("failed to update verity args for UKI (%s):\n%w", ukiFile, err)
		}
	}

	err = espMount.CleanClose()
	if err != nil {
		return err
	}

	return nil
}

// updateUkiFileVerityArgs updates verity args in a single UKI file's addons.
// It checks if verity args are in the main UKI (error if so — can't modify signed PE),
// then updates each addon by stripping old verity args and appending new ones.
func updateUkiFileVerityArgs(ukiFile string, buildDir string, addonStubPath string,
	newVerityArgsStr string,
) error {
	ukiFileName := filepath.Base(ukiFile)
	kernelName := strings.TrimSuffix(ukiFileName, ".efi")

	// Check main UKI for verity args — if present, we can't safely modify them.
	mainCmdline, err := extractCmdlineFromSinglePE(ukiFile, buildDir)
	if err != nil {
		return fmt.Errorf("failed to extract cmdline from main UKI:\n%w", err)
	}

	strippedMain := removeVerityArgsFromCmdline(mainCmdline)
	if strings.TrimSpace(strippedMain) != strings.TrimSpace(mainCmdline) {
		return fmt.Errorf("%w: verity args found in main UKI %s", ErrConvertNewUuidsUkiMainSigned, ukiFile)
	}

	// Process addons.
	addonDirPath := filepath.Join(filepath.Dir(ukiFile), fmt.Sprintf("%s.extra.d", ukiFileName))

	entries, err := os.ReadDir(addonDirPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No addon directory — create one with the verity args.
			return createVerityAddon(addonDirPath, kernelName, addonStubPath, newVerityArgsStr)
		}
		return fmt.Errorf("failed to read addon dir:\n%w", err)
	}

	updatedAny := false
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".addon.efi") {
			continue
		}

		addonPath := filepath.Join(addonDirPath, entry.Name())
		addonCmdline, err := extractCmdlineFromSinglePE(addonPath, buildDir)
		if err != nil {
			return fmt.Errorf("failed to extract cmdline from addon %s:\n%w", addonPath, err)
		}

		stripped := removeVerityArgsFromCmdline(addonCmdline)
		if strings.TrimSpace(stripped) == strings.TrimSpace(addonCmdline) {
			// No verity args in this addon — skip.
			continue
		}

		// Rebuild this addon with stripped cmdline + new verity args.
		var newCmdline string
		if strings.TrimSpace(stripped) != "" {
			newCmdline = strings.TrimSpace(stripped) + " " + newVerityArgsStr
		} else {
			newCmdline = newVerityArgsStr
		}

		err = rebuildAddon(addonPath, addonStubPath, newCmdline)
		if err != nil {
			return err
		}
		updatedAny = true
	}

	if !updatedAny {
		// Verity args weren't in any existing addon — create a new one.
		return createVerityAddon(addonDirPath, kernelName, addonStubPath, newVerityArgsStr)
	}

	return nil
}

// createVerityAddon creates a new addon.efi file containing the verity kernel args.
func createVerityAddon(addonDirPath string, kernelName string, addonStubPath string,
	verityArgsStr string,
) error {
	err := os.MkdirAll(addonDirPath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create addon directory:\n%w", err)
	}

	addonPath := filepath.Join(addonDirPath, fmt.Sprintf("%s.addon.efi", kernelName))
	return rebuildAddon(addonPath, addonStubPath, verityArgsStr)
}

// rebuildAddon rebuilds a UKI addon PE file with the given cmdline using ukify.
func rebuildAddon(addonPath string, stubPath string, cmdline string) error {
	logger.Log.Infof("Rebuilding UKI addon: %s", addonPath)

	ukifyCmd := []string{
		"build",
		fmt.Sprintf("--cmdline=%s", cmdline),
		fmt.Sprintf("--stub=%s", stubPath),
		fmt.Sprintf("--output=%s", addonPath),
	}

	err := shell.ExecuteLiveWithErr(1, "ukify", ukifyCmd...)
	if err != nil {
		return fmt.Errorf("failed to rebuild UKI addon (%s):\n%w", addonPath, err)
	}

	return nil
}
