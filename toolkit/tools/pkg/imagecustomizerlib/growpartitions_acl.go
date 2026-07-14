// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
)

// ACL's fixed, well-known GPT partition labels, in on-disk order.
const (
	aclPartLabelEsp  = "EFI-SYSTEM"
	aclPartLabelUsrA = "USR-A"
	aclPartLabelUsrB = "USR-B"
	aclPartLabelOem  = "OEM"
	aclPartLabelRoot = "ROOT"
)

// aclStandardPartLabelsInOrder is the exact, sealed base layout ACL ships. The grow API only
// operates on images that match this layout exactly.
var aclStandardPartLabelsInOrder = []string{
	aclPartLabelEsp, aclPartLabelUsrA, aclPartLabelUsrB, aclPartLabelOem, aclPartLabelRoot,
}

var (
	ErrAclGrowUnexpectedLayout = NewImageCustomizerError("AclGrow:UnexpectedLayout",
		"image does not match the expected ACL standard partition layout")
	ErrAclGrowShrinkRequested = NewImageCustomizerError("AclGrow:ShrinkRequested",
		"requested size is smaller than the current partition size (grow-only)")
	ErrAclGrowParseTable = NewImageCustomizerError("AclGrow:ParseTable",
		"failed to parse partition table")
	ErrAclGrowClone = NewImageCustomizerError("AclGrow:Clone",
		"failed to clone image into grown layout")
	ErrAclGrowFilesystem = NewImageCustomizerError("AclGrow:Filesystem",
		"failed to grow filesystem")
)

// sfdiskKeyValueRegex matches `key=value` pairs in an `sfdisk --dump` partition line, where value
// is either a double-quoted string (which may contain commas, e.g. attrs="GUID:48,56") or an
// unquoted, comma-free run.
var sfdiskKeyValueRegex = regexp.MustCompile(`([A-Za-z][A-Za-z0-9_-]*)=("[^"]*"|[^,]*)`)

// aclPartitionEntry is one partition line from an `sfdisk --dump`, with its ordered key=value
// fields preserved so the entry's identity (type, uuid, name, attrs, ...) round-trips exactly.
type aclPartitionEntry struct {
	fields    []sfdiskField
	startSect uint64
	sizeSect  uint64
	label     string
}

type sfdiskField struct {
	key    string
	value  string // Includes surrounding quotes when the original value was quoted.
	quoted bool
}

// aclPartitionTable is a parsed `sfdisk --dump`: header metadata plus the ordered partition list.
type aclPartitionTable struct {
	labelId    string
	firstLba   uint64
	sectorSize uint64
	partitions []*aclPartitionEntry
}

// growAclStandardPartitions clones baseImageFile into newImageFile, growing the requested ACL
// standard partitions to their target sizes. It operates purely at the GPT/block level: it
// preserves every partition's type GUID, PARTUUID, label, and GPT attribute bits (the systemd A/B
// bits) exactly, and copies all partition content verbatim. Growth is absorbed by enlarging the
// total disk by exactly the growth delta, so the trailing ROOT partition keeps its original size
// and is merely shifted to a later offset (never shrunk). ESP (vfat) is recreated at the larger
// size preserving its volume id, label, and files. The btrfs /usr filesystem is NOT resized here;
// that happens after the image is connected (see growAclUsrFilesystem), so the base verity
// superblock stays intact for base-image verity discovery.
func growAclStandardPartitions(ctx context.Context, acl *imagecustomizerapi.Acl, baseImageFile string,
	newImageFile string,
) error {
	logger.Log.Infof("Growing ACL standard partitions")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "grow_acl_partitions")
	defer span.End()

	baseLoopback, err := safeloopback.NewLoopback(baseImageFile)
	if err != nil {
		return err
	}
	defer baseLoopback.Close()

	table, partitions, err := readAclPartitionTable(baseLoopback.DevicePath())
	if err != nil {
		return err
	}

	// Compute the requested new sizes per label and validate grow-only.
	requestedSizes, err := resolveAclRequestedSizes(acl, table)
	if err != nil {
		return err
	}

	if len(requestedSizes) == 0 {
		// Every requested partition already has the requested size: nothing to do.
		// Caller falls back to the original image; signal via a sentinel.
		return errAclGrowNoOp
	}

	// Recompute the new layout. This grows the requested partitions, shifts the following ones
	// right, and keeps the trailing ROOT partition at its original size (so ROOT is merely moved,
	// never shrunk). The total growth (in bytes) is returned so the disk is enlarged to match.
	growthBytes, espRecreated := applyAclGrownLayout(table, requestedSizes)

	newDiskBytes := aclAlignedDiskSize(baseImageFile, growthBytes)

	// Create the new disk file and restore the edited GPT.
	err = diskutils.CreateSparseDisk(newImageFile, newDiskBytes/diskutils.MiB, 0o644)
	if err != nil {
		return fmt.Errorf("%w:\nfailed to create new disk file:\n%w", ErrAclGrowClone, err)
	}

	newLoopback, err := safeloopback.NewLoopback(newImageFile)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrAclGrowClone, err)
	}
	defer newLoopback.Close()

	err = restoreAclPartitionTable(newLoopback.DevicePath(), table)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrAclGrowClone, err)
	}

	err = diskutils.RefreshPartitions(newLoopback.DevicePath())
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrAclGrowClone, err)
	}

	newPartitions, err := diskutils.GetDiskPartitions(newLoopback.DevicePath())
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrAclGrowClone, err)
	}

	// Copy every partition's content verbatim.
	err = cloneAclPartitionContents(partitions, newPartitions, requestedSizes, espRecreated)
	if err != nil {
		return err
	}

	// Recreate the ESP vfat at the larger size, preserving volume id, label, and files.
	if espRecreated {
		err = recreateAclEspFilesystem(newLoopback.DevicePath(), newPartitions)
		if err != nil {
			return err
		}
	}

	err = newLoopback.CleanClose()
	if err != nil {
		return err
	}

	err = baseLoopback.CleanClose()
	if err != nil {
		return err
	}

	return nil
}

// errAclGrowNoOp signals that the requested grow is a no-op (all requested sizes already match).
var errAclGrowNoOp = fmt.Errorf("acl grow is a no-op")

// readAclPartitionTable parses the disk's GPT via `sfdisk --dump` and validates that the disk
// matches ACL's exact standard layout. It returns the parsed table and the current lsblk
// partition info (for size/label lookups).
func readAclPartitionTable(diskDevPath string) (*aclPartitionTable, []diskutils.PartitionInfo, error) {
	dump, _, err := shell.Execute("sfdisk", "--dump", diskDevPath)
	if err != nil {
		return nil, nil, fmt.Errorf("%w:\n%w", ErrAclGrowParseTable, err)
	}

	table, err := parseAclPartitionTable(dump)
	if err != nil {
		return nil, nil, fmt.Errorf("%w:\n%w", ErrAclGrowParseTable, err)
	}

	// Validate the exact ACL layout: same number of partitions, same labels, same order.
	if len(table.partitions) != len(aclStandardPartLabelsInOrder) {
		return nil, nil, fmt.Errorf("%w: expected %d partitions, found %d", ErrAclGrowUnexpectedLayout,
			len(aclStandardPartLabelsInOrder), len(table.partitions))
	}
	for i, expectedLabel := range aclStandardPartLabelsInOrder {
		if table.partitions[i].label != expectedLabel {
			return nil, nil, fmt.Errorf("%w: partition %d has label '%s', expected '%s'",
				ErrAclGrowUnexpectedLayout, i+1, table.partitions[i].label, expectedLabel)
		}
	}

	partitions, err := diskutils.GetDiskPartitions(diskDevPath)
	if err != nil {
		return nil, nil, err
	}

	return table, partitions, nil
}

// parseAclPartitionTable parses the text output of `sfdisk --dump`.
func parseAclPartitionTable(dump string) (*aclPartitionTable, error) {
	table := &aclPartitionTable{sectorSize: 512}

	for _, line := range strings.Split(dump, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Header lines are `key: value`; partition lines are `node : key=value, ...`.
		if !strings.Contains(trimmed, "=") {
			key, value, found := strings.Cut(trimmed, ":")
			if !found {
				continue
			}
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			switch key {
			case "label-id":
				table.labelId = value
			case "first-lba":
				table.firstLba, _ = strconv.ParseUint(value, 10, 64)
			case "sector-size":
				if v, err := strconv.ParseUint(value, 10, 64); err == nil && v != 0 {
					table.sectorSize = v
				}
			}
			continue
		}

		entry, err := parseAclPartitionLine(trimmed)
		if err != nil {
			return nil, err
		}
		table.partitions = append(table.partitions, entry)
	}

	if len(table.partitions) == 0 {
		return nil, fmt.Errorf("no partitions found in partition table dump")
	}

	return table, nil
}

// parseAclPartitionLine parses a single `sfdisk --dump` partition line, preserving field order.
func parseAclPartitionLine(line string) (*aclPartitionEntry, error) {
	// Strip the `node :` prefix; the remainder is the comma-separated key=value list.
	_, rest, found := strings.Cut(line, ":")
	if !found {
		return nil, fmt.Errorf("unexpected partition line format: %s", line)
	}

	entry := &aclPartitionEntry{}
	matches := sfdiskKeyValueRegex.FindAllStringSubmatch(rest, -1)
	for _, match := range matches {
		key := match[1]
		rawValue := strings.TrimSpace(match[2])
		quoted := strings.HasPrefix(rawValue, "\"")

		entry.fields = append(entry.fields, sfdiskField{key: key, value: rawValue, quoted: quoted})

		switch key {
		case "start":
			entry.startSect, _ = strconv.ParseUint(rawValue, 10, 64)
		case "size":
			entry.sizeSect, _ = strconv.ParseUint(rawValue, 10, 64)
		case "name":
			entry.label = strings.Trim(rawValue, "\"")
		}
	}

	if entry.label == "" {
		return nil, fmt.Errorf("partition line has no name: %s", line)
	}

	return entry, nil
}

// resolveAclRequestedSizes maps ACL partition labels to their requested new size in bytes, after
// validating grow-only semantics. Partitions already at the requested size are omitted (no-op).
func resolveAclRequestedSizes(acl *imagecustomizerapi.Acl, table *aclPartitionTable,
) (map[string]uint64, error) {
	// Map label -> current size in bytes.
	currentSizes := make(map[string]uint64)
	for _, p := range table.partitions {
		currentSizes[p.label] = p.sizeSect * table.sectorSize
	}

	// Build the set of (label -> requested size). USR grows both A and B to the same size.
	type request struct {
		labels []string
		size   uint64
	}
	var requests []request
	if acl.Usr != nil {
		requests = append(requests, request{
			labels: []string{aclPartLabelUsrA, aclPartLabelUsrB},
			size:   uint64(acl.Usr.Size),
		})
	}
	if acl.Esp != nil {
		requests = append(requests, request{
			labels: []string{aclPartLabelEsp},
			size:   uint64(acl.Esp.Size),
		})
	}

	result := make(map[string]uint64)
	for _, req := range requests {
		for _, label := range req.labels {
			current, ok := currentSizes[label]
			if !ok {
				return nil, fmt.Errorf("%w: partition '%s' not found", ErrAclGrowUnexpectedLayout, label)
			}
			if req.size < current {
				return nil, fmt.Errorf("%w: partition '%s' current size is %s, requested %s",
					ErrAclGrowShrinkRequested, label,
					imagecustomizerapi.DiskSize(current).HumanReadable(),
					imagecustomizerapi.DiskSize(req.size).HumanReadable())
			}
			if req.size == current {
				// No-op for this partition.
				continue
			}
			result[label] = req.size
		}
	}

	return result, nil
}

// applyAclGrownLayout rewrites the table's partition starts/sizes to grow the requested partitions
// and re-pack all partitions contiguously. Every partition (including the trailing ROOT) keeps its
// original size except the ones being grown; growth is absorbed by enlarging the total disk, so
// ROOT is merely shifted to a later offset, never shrunk. Returns the total growth in bytes and
// whether the ESP was grown (and must be recreated).
func applyAclGrownLayout(table *aclPartitionTable, requestedSizes map[string]uint64) (uint64, bool) {
	espRecreated := false
	var growthSectors uint64

	var nextStart uint64
	for i, p := range table.partitions {
		if i == 0 {
			nextStart = p.startSect
		}

		setAclField(p, "start", strconv.FormatUint(nextStart, 10))
		p.startSect = nextStart

		if newSize, grow := requestedSizes[p.label]; grow {
			newSizeSect := newSize / table.sectorSize
			// Accumulate growth using the original size, before mutating it.
			growthSectors += newSizeSect - p.sizeSect
			setAclField(p, "size", strconv.FormatUint(newSizeSect, 10))
			p.sizeSect = newSizeSect
			if p.label == aclPartLabelEsp {
				espRecreated = true
			}
		}

		nextStart = p.startSect + p.sizeSect
	}

	return growthSectors * table.sectorSize, espRecreated
}

func setAclField(entry *aclPartitionEntry, key string, value string) {
	for i := range entry.fields {
		if entry.fields[i].key == key {
			entry.fields[i].value = value
			entry.fields[i].quoted = false
			return
		}
	}
	entry.fields = append(entry.fields, sfdiskField{key: key, value: value})
}

// aclAlignedDiskSize computes the size of the grown disk: the base disk size plus the total
// partition growth, aligned up to a whole MiB. Enlarging the disk by exactly the growth means the
// trailing ROOT partition keeps its original size and is simply shifted to a later offset.
func aclAlignedDiskSize(baseImageFile string, growthBytes uint64) uint64 {
	stat, err := os.Stat(baseImageFile)
	baseBytes := uint64(0)
	if err == nil {
		baseBytes = uint64(stat.Size())
	}

	total := baseBytes + growthBytes
	// Align up to MiB.
	if rem := total % diskutils.MiB; rem != 0 {
		total += diskutils.MiB - rem
	}
	return total
}

// restoreAclPartitionTable serializes the (edited) table and restores it onto the disk with sfdisk.
func restoreAclPartitionTable(diskDevPath string, table *aclPartitionTable) error {
	script := buildAclSfdiskScript(table)

	err := shell.NewExecBuilder("sfdisk", diskDevPath).
		Stdin(script).
		LogLevel(logrus.DebugLevel, logrus.WarnLevel).
		ErrorStderrLines(1).
		Execute()
	if err != nil {
		return fmt.Errorf("failed to restore partition table with sfdisk:\n%w", err)
	}

	return nil
}

// buildAclSfdiskScript serializes the (edited) table into an sfdisk restore script. The last-lba
// header is intentionally omitted so sfdisk sizes the layout for the (larger) target disk.
func buildAclSfdiskScript(table *aclPartitionTable) string {
	var sb strings.Builder
	sb.WriteString("label: gpt\n")
	if table.labelId != "" {
		sb.WriteString(fmt.Sprintf("label-id: %s\n", table.labelId))
	}
	sb.WriteString("unit: sectors\n")
	if table.firstLba != 0 {
		sb.WriteString(fmt.Sprintf("first-lba: %d\n", table.firstLba))
	}
	sb.WriteString("\n")

	for _, p := range table.partitions {
		parts := make([]string, 0, len(p.fields))
		for _, f := range p.fields {
			parts = append(parts, fmt.Sprintf("%s=%s", f.key, f.value))
		}
		sb.WriteString(strings.Join(parts, ", "))
		sb.WriteString("\n")
	}

	return sb.String()
}

// cloneAclPartitionContents copies each partition's content verbatim from the base disk to the new
// disk. ESP is skipped when it will be recreated (its content is preserved separately).
func cloneAclPartitionContents(oldPartitions []diskutils.PartitionInfo,
	newPartitions []diskutils.PartitionInfo, requestedSizes map[string]uint64, espRecreated bool,
) error {
	oldByLabel := partitionsByLabel(oldPartitions)
	newByLabel := partitionsByLabel(newPartitions)

	for _, label := range aclStandardPartLabelsInOrder {
		if label == aclPartLabelEsp && espRecreated {
			// The ESP is preserved via recreateAclEspFilesystem instead of a raw copy.
			continue
		}

		oldPart, ok := oldByLabel[label]
		if !ok {
			return fmt.Errorf("%w: base partition '%s' not found", ErrAclGrowClone, label)
		}
		newPart, ok := newByLabel[label]
		if !ok {
			return fmt.Errorf("%w: new partition '%s' not found", ErrAclGrowClone, label)
		}

		err := shell.NewExecBuilder("dd", "if="+oldPart.Path, "of="+newPart.Path,
			"bs=1M", "conv=fsync", "status=none").
			LogLevel(logrus.DebugLevel, logrus.WarnLevel).
			ErrorStderrLines(1).
			Execute()
		if err != nil {
			return fmt.Errorf("%w: failed to copy partition '%s':\n%w", ErrAclGrowClone, label, err)
		}
	}

	return nil
}

func partitionsByLabel(partitions []diskutils.PartitionInfo) map[string]diskutils.PartitionInfo {
	result := make(map[string]diskutils.PartitionInfo)
	for _, p := range partitions {
		if p.Type == "part" && p.PartLabel != "" {
			result[p.PartLabel] = p
		}
	}
	return result
}

// recreateAclEspFilesystem recreates the ESP vfat filesystem at the enlarged partition size,
// preserving its volume id, label, and files. FAT cannot be grown in place without fatresize
// (not a toolkit dependency), so the ESP is copied out, reformatted, and copied back in.
func recreateAclEspFilesystem(diskDevPath string, newPartitions []diskutils.PartitionInfo) error {
	espPart, ok := partitionsByLabel(newPartitions)[aclPartLabelEsp]
	if !ok {
		return fmt.Errorf("%w: ESP partition not found on new disk", ErrAclGrowFilesystem)
	}

	tmpDir, err := os.MkdirTemp("", "acl-esp-")
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrAclGrowFilesystem, err)
	}
	defer os.RemoveAll(tmpDir)

	stageDir := filepath.Join(tmpDir, "stage")
	mountDir := filepath.Join(tmpDir, "mnt")

	// Read the current volume id and label so the reformatted ESP keeps the same identity.
	volumeId, label, err := readVfatIdentity(espPart.Path)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrAclGrowFilesystem, err)
	}

	// Copy the existing ESP files out.
	espMount, err := safemount.NewMount(espPart.Path, mountDir, "vfat", 0, "", true)
	if err != nil {
		return fmt.Errorf("%w: failed to mount existing ESP:\n%w", ErrAclGrowFilesystem, err)
	}
	err = os.MkdirAll(stageDir, 0o755)
	if err != nil {
		espMount.Close()
		return fmt.Errorf("%w:\n%w", ErrAclGrowFilesystem, err)
	}
	err = copyPartitionFilesWithOptions(mountDir+"/.", stageDir, false /*noClobber*/)
	if err != nil {
		espMount.Close()
		return fmt.Errorf("%w: failed to stage ESP files:\n%w", ErrAclGrowFilesystem, err)
	}
	err = espMount.CleanClose()
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrAclGrowFilesystem, err)
	}

	// Reformat the (larger) ESP, preserving volume id and label.
	mkfsArgs := []string{"-F", "32"}
	if volumeId != "" {
		mkfsArgs = append(mkfsArgs, "-i", volumeId)
	}
	if label != "" {
		mkfsArgs = append(mkfsArgs, "-n", label)
	}
	mkfsArgs = append(mkfsArgs, espPart.Path)
	err = shell.NewExecBuilder("mkfs.vfat", mkfsArgs...).
		LogLevel(logrus.DebugLevel, logrus.WarnLevel).
		ErrorStderrLines(1).
		Execute()
	if err != nil {
		return fmt.Errorf("%w: mkfs.vfat failed on ESP:\n%w", ErrAclGrowFilesystem, err)
	}

	// Copy the files back in.
	espMount, err = safemount.NewMount(espPart.Path, mountDir, "vfat", 0, "", false)
	if err != nil {
		return fmt.Errorf("%w: failed to remount ESP:\n%w", ErrAclGrowFilesystem, err)
	}
	err = copyPartitionFilesWithOptions(stageDir+"/.", mountDir, false /*noClobber*/)
	if err != nil {
		espMount.Close()
		return fmt.Errorf("%w: failed to restore ESP files:\n%w", ErrAclGrowFilesystem, err)
	}
	err = espMount.CleanClose()
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrAclGrowFilesystem, err)
	}

	return nil
}

// readVfatIdentity returns the vfat volume id (as an 8-hex-digit string suitable for `mkfs.vfat -i`)
// and the filesystem label.
func readVfatIdentity(partitionPath string) (volumeId string, label string, err error) {
	uuidOut, _, err := shell.Execute("blkid", "-s", "UUID", "-o", "value", partitionPath)
	if err == nil {
		// vfat UUID is formatted as "ABCD-1234"; mkfs.vfat -i wants "ABCD1234".
		volumeId = strings.ReplaceAll(strings.TrimSpace(uuidOut), "-", "")
	}

	labelOut, _, err := shell.Execute("blkid", "-s", "LABEL", "-o", "value", partitionPath)
	if err == nil {
		label = strings.TrimSpace(labelOut)
	}

	// blkid returning no LABEL is not an error.
	return volumeId, label, nil
}

// growAclUsrFilesystem grows the active /usr btrfs filesystem to the inline-verity data size that
// fits within the enlarged USR partition (leaving room for the re-generated verity hash tree). It
// must run after the image is connected (so /usr is mounted read-write) and before package
// installs. Only the active /usr (the mounted one) is grown; the A/B second copy (USR-B) keeps its
// original, self-consistent verity seal.
func growAclUsrFilesystem(imageConnection *imageconnection.ImageConnection) error {
	usrDir := filepath.Join(imageConnection.Chroot().RootDir(), "usr")

	partitions, err := diskutils.GetDiskPartitions(imageConnection.Loopback().DevicePath())
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrAclGrowFilesystem, err)
	}

	usrPart, ok := findAclUsrPartition(partitions)
	if !ok {
		return fmt.Errorf("%w: could not find mounted /usr partition", ErrAclGrowFilesystem)
	}

	newDataSize, err := imagecustomizerapi.CalculateInlineVerityDataSize(usrPart.SizeInBytes)
	if err != nil {
		return fmt.Errorf("%w: failed to compute /usr inline verity data size:\n%w", ErrAclGrowFilesystem, err)
	}

	logger.Log.Infof("Growing /usr btrfs filesystem to %s (partition %s, leaving room for verity hash tree)",
		imagecustomizerapi.DiskSize(newDataSize).HumanReadable(),
		imagecustomizerapi.DiskSize(usrPart.SizeInBytes).HumanReadable())

	err = shell.NewExecBuilder("btrfs", "filesystem", "resize", strconv.FormatUint(newDataSize, 10), usrDir).
		LogLevel(logrus.DebugLevel, logrus.WarnLevel).
		ErrorStderrLines(1).
		Execute()
	if err != nil {
		return fmt.Errorf("%w: btrfs resize failed on /usr:\n%w", ErrAclGrowFilesystem, err)
	}

	return nil
}

// aclUsrVerityDataSize returns the inline-verity data size that the grown USR partition should be
// sealed at. Used to override the base-image verity metadata so verity is re-sealed at the new
// offset and the UKI cmdline gets the new hash-offset.
func aclUsrVerityDataSize(rawImageFile string) (uint64, error) {
	loopback, err := safeloopback.NewLoopback(rawImageFile)
	if err != nil {
		return 0, err
	}
	defer loopback.Close()

	partitions, err := diskutils.GetDiskPartitions(loopback.DevicePath())
	if err != nil {
		return 0, err
	}

	usrPart, ok := findAclUsrPartition(partitions)
	if !ok {
		// Fall back to matching by label when nothing is mounted.
		byLabel := partitionsByLabel(partitions)
		usrPart, ok = byLabel[aclPartLabelUsrA]
		if !ok {
			return 0, fmt.Errorf("%w: could not find USR partition", ErrAclGrowFilesystem)
		}
	}

	dataSize, err := imagecustomizerapi.CalculateInlineVerityDataSize(usrPart.SizeInBytes)
	if err != nil {
		return 0, err
	}

	err = loopback.CleanClose()
	if err != nil {
		return 0, err
	}

	return dataSize, nil
}

// findAclUsrPartition returns the USR partition that is mounted at (or under) /usr, falling back to
// the USR-A labelled partition.
func findAclUsrPartition(partitions []diskutils.PartitionInfo) (diskutils.PartitionInfo, bool) {
	for _, p := range partitions {
		if p.Type == "part" && strings.HasSuffix(p.Mountpoint, "/usr") {
			return p, true
		}
	}
	if p, ok := partitionsByLabel(partitions)[aclPartLabelUsrA]; ok {
		return p, true
	}
	return diskutils.PartitionInfo{}, false
}

// overrideAclUsrVerityMetadata rewrites the USR verity device's inline data size and hash offset to
// match the grown /usr partition, so the subsequent verity re-seal formats the hash tree at the new
// offset and the rebuilt UKI cmdline carries the matching hash-offset.
func overrideAclUsrVerityMetadata(rawImageFile string, verityMetadata []verityDeviceMetadata) error {
	dataSize, err := aclUsrVerityDataSize(rawImageFile)
	if err != nil {
		return err
	}

	found := false
	for i := range verityMetadata {
		if verityMetadata[i].name == imagecustomizerapi.VerityUsrDeviceName {
			verityMetadata[i].formatSettings.dataSizeBytes = dataSize
			verityMetadata[i].formatSettings.hashOffsetBytes = dataSize
			found = true
		}
	}

	if !found {
		return fmt.Errorf("%w: no /usr verity device found to re-seal after grow", ErrAclGrowFilesystem)
	}

	return nil
}
