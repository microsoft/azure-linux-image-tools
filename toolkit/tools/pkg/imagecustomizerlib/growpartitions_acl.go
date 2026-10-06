// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/mathutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/verityutils"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
)

// ACL's well-known GPT partition labels.
const (
	aclPartLabelEsp      = "EFI-SYSTEM"
	aclPartLabelUsrA     = "USR-A"
	aclPartLabelUsrB     = "USR-B"
	aclPartLabelHashA    = "HASH-A"
	aclPartLabelHashB    = "HASH-B"
	aclPartLabelHashSigA = "HASH-SIG-A"
	aclPartLabelHashSigB = "HASH-SIG-B"
	aclPartLabelOem      = "OEM"
	aclPartLabelRoot     = "ROOT"
)

// aclRequiredPartLabels must be present for an image to be recognized as an ACL image the grow API
// can operate on. Order is deliberately not asserted: ACL has already renumbered its partitions
// once (per-slot verity hash partitions were inserted after each USR, moving OEM and ROOT), and
// pinning a sequence turns every future layout change into a hard failure.
var aclRequiredPartLabels = []string{
	aclPartLabelEsp, aclPartLabelUsrA, aclPartLabelUsrB, aclPartLabelOem, aclPartLabelRoot,
}

// aclUsrHashPartLabels maps each USR partition to its dedicated dm-verity hash partition, for
// layouts that have them. Images without these partitions seal /usr with inline verity instead
// (the hash tree lives at an offset inside the USR partition).
var aclUsrHashPartLabels = map[string]string{
	aclPartLabelUsrA: aclPartLabelHashA,
	aclPartLabelUsrB: aclPartLabelHashB,
}

// aclOptionalPartLabels are recognized but not required. Hash signature partitions are a proposed
// addition (azure-container-linux#88) and are accepted ahead of that change landing.
var aclOptionalPartLabels = []string{
	aclPartLabelHashA, aclPartLabelHashB, aclPartLabelHashSigA, aclPartLabelHashSigB,
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
	// cloneAclPartitionContents deliberately skipped the ESP, so the new ESP partition is empty; the
	// files and vfat identity are read from the base (old) ESP, which is still attached at this point.
	if espRecreated {
		oldEsp, ok := partitionsByLabel(partitions)[aclPartLabelEsp]
		if !ok {
			return fmt.Errorf("%w: base ESP partition not found", ErrAclGrowFilesystem)
		}
		newEsp, ok := partitionsByLabel(newPartitions)[aclPartLabelEsp]
		if !ok {
			return fmt.Errorf("%w: new ESP partition not found", ErrAclGrowFilesystem)
		}
		err = recreateAclEspFilesystem(oldEsp.Path, newEsp.Path)
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

	// Validate by role rather than by sequence: every required partition must be present, but the
	// on-disk order and the presence of optional partitions are not constrained.
	err = validateAclLayout(table)
	if err != nil {
		return nil, nil, err
	}

	partitions, err := diskutils.GetDiskPartitions(diskDevPath)
	if err != nil {
		return nil, nil, err
	}

	return table, partitions, nil
}

// validateAclLayout checks that the table holds every partition the grow API requires, and reports
// what kind of /usr verity sealing the image uses. Partitions that are neither required nor
// recognized are left alone (and cloned verbatim) rather than rejected, so that a future ACL layout
// addition does not block customization outright.
func validateAclLayout(table *aclPartitionTable) error {
	byLabel := make(map[string]bool, len(table.partitions))
	for _, p := range table.partitions {
		if p.label != "" {
			byLabel[p.label] = true
		}
	}

	var missing []string
	for _, label := range aclRequiredPartLabels {
		if !byLabel[label] {
			missing = append(missing, label)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: missing required partition(s): %s (found: %s)",
			ErrAclGrowUnexpectedLayout, strings.Join(missing, ", "), describeAclLayout(table))
	}

	// A partition carrying a USR's hash must not appear without its USR, and vice versa, or the
	// pair cannot be grown coherently.
	for usrLabel, hashLabel := range aclUsrHashPartLabels {
		if byLabel[hashLabel] && !byLabel[usrLabel] {
			return fmt.Errorf("%w: '%s' is present without '%s'", ErrAclGrowUnexpectedLayout,
				hashLabel, usrLabel)
		}
	}

	if !aclHasDedicatedUsrHashPartitions(table) {
		logger.Log.Warnf("This ACL image seals /usr with inline dm-verity (no %s/%s partitions). That "+
			"layout is deprecated; current ACL images use dedicated per-slot verity hash partitions. "+
			"Support for inline /usr verity will be removed in a future release.",
			aclPartLabelHashA, aclPartLabelHashB)
	}

	for _, p := range table.partitions {
		if p.label == "" {
			continue
		}
		if slices.Contains(aclRequiredPartLabels, p.label) || slices.Contains(aclOptionalPartLabels, p.label) {
			continue
		}
		logger.Log.Infof("Partition '%s' is not part of the known ACL layout; it will be copied "+
			"unchanged and never resized", p.label)
	}

	return nil
}

// aclHasDedicatedUsrHashPartitions reports whether the image carries dedicated dm-verity hash
// partitions for /usr. When it does, /usr fills its whole partition and the hash tree lives in the
// companion partition; otherwise /usr is sealed with inline verity.
func aclHasDedicatedUsrHashPartitions(table *aclPartitionTable) bool {
	for _, p := range table.partitions {
		if p.label == aclPartLabelHashA {
			return true
		}
	}
	return false
}

// describeAclLayout renders the table's labels for error messages.
func describeAclLayout(table *aclPartitionTable) string {
	labels := make([]string, 0, len(table.partitions))
	for _, p := range table.partitions {
		if p.label == "" {
			labels = append(labels, "<unlabelled>")
			continue
		}
		labels = append(labels, p.label)
	}
	return strings.Join(labels, ", ")
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
		// derived requests are computed by IC rather than asked for by the user, so a computed
		// size at or below the current size means "already big enough", not an error.
		derived bool
	}
	var requests []request
	if acl.Usr != nil {
		requests = append(requests, request{
			labels: []string{aclPartLabelUsrA, aclPartLabelUsrB},
			size:   uint64(acl.Usr.Size),
		})

		// On layouts with dedicated verity hash partitions, the hash tree must grow with the data
		// it covers, so size the companion partitions from the requested /usr size rather than
		// asking the user to compute it.
		if aclHasDedicatedUsrHashPartitions(table) {
			hashSize, err := aclUsrHashPartitionSize(uint64(acl.Usr.Size))
			if err != nil {
				return nil, err
			}

			var hashLabels []string
			for _, usrLabel := range []string{aclPartLabelUsrA, aclPartLabelUsrB} {
				hashLabel := aclUsrHashPartLabels[usrLabel]
				if _, ok := currentSizes[hashLabel]; ok {
					hashLabels = append(hashLabels, hashLabel)
				}
			}

			if len(hashLabels) > 0 {
				logger.Log.Infof("Sizing /usr verity hash partitions to %s for a %s /usr",
					imagecustomizerapi.DiskSize(hashSize).HumanReadable(),
					imagecustomizerapi.DiskSize(acl.Usr.Size).HumanReadable())
				requests = append(requests, request{labels: hashLabels, size: hashSize, derived: true})
			}
		}
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
				if req.derived {
					// Already larger than required; leave it untouched (grow-only).
					continue
				}
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

	// Iterate the partitions actually present on the base disk rather than a fixed list, so that
	// partitions outside the known ACL layout (e.g. verity hash or hash signature partitions) are
	// carried over instead of being silently left empty in the grown image.
	for _, oldPart := range oldPartitions {
		label := oldPart.PartLabel
		if oldPart.Type != "part" || label == "" {
			continue
		}
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
			return fmt.Errorf("%w: grown image has no partition '%s' to copy into", ErrAclGrowClone, label)
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
// (not a toolkit dependency), so the files and identity are read from the base (old) ESP, the new
// (larger, empty) ESP partition is formatted fresh, and the files are copied back in.
func recreateAclEspFilesystem(oldEspPath string, newEspPath string) error {
	tmpDir, err := os.MkdirTemp("", "acl-esp-")
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrAclGrowFilesystem, err)
	}
	defer os.RemoveAll(tmpDir)

	stageDir := filepath.Join(tmpDir, "stage")
	mountDir := filepath.Join(tmpDir, "mnt")

	// Read the base ESP's volume id and label so the reformatted ESP keeps the same identity.
	volumeId, label, err := readVfatIdentity(oldEspPath)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrAclGrowFilesystem, err)
	}

	// Copy the existing files out of the base (old) ESP.
	espMount, err := safemount.NewMount(oldEspPath, mountDir, "vfat", 0, "", true)
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

	// Reformat the new (larger) ESP, preserving volume id and label.
	mkfsArgs := []string{"-F", "32"}
	if volumeId != "" {
		mkfsArgs = append(mkfsArgs, "-i", volumeId)
	}
	if label != "" {
		mkfsArgs = append(mkfsArgs, "-n", label)
	}
	mkfsArgs = append(mkfsArgs, newEspPath)
	err = shell.NewExecBuilder("mkfs.vfat", mkfsArgs...).
		LogLevel(logrus.DebugLevel, logrus.WarnLevel).
		ErrorStderrLines(1).
		Execute()
	if err != nil {
		return fmt.Errorf("%w: mkfs.vfat failed on ESP:\n%w", ErrAclGrowFilesystem, err)
	}

	// Copy the files back in to the new ESP.
	espMount, err = safemount.NewMount(newEspPath, mountDir, "vfat", 0, "", true)
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

// growAclUsrFilesystem grows the active /usr btrfs filesystem into its enlarged partition. It must
// run after the image is connected (so /usr is mounted read-write) and before package installs.
// Only the active /usr (the mounted one) is grown; the A/B second copy keeps its original,
// self-consistent verity seal.
//
// How much of the partition the filesystem may use depends on where the verity hash tree lives: on
// layouts with a dedicated hash partition the filesystem fills the partition, while with inline
// verity it must stop short to leave room for the hash tree written after it.
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

	newDataSize, inlineVerity, err := aclUsrFilesystemSize(partitions, usrPart)
	if err != nil {
		return err
	}

	if inlineVerity {
		logger.Log.Infof("Growing /usr btrfs filesystem to %s (partition %s, leaving room for the "+
			"inline verity hash tree)",
			imagecustomizerapi.DiskSize(newDataSize).HumanReadable(),
			imagecustomizerapi.DiskSize(usrPart.SizeInBytes).HumanReadable())
	} else {
		logger.Log.Infof("Growing /usr btrfs filesystem to %s (fills the partition; the verity hash "+
			"tree has its own partition)",
			imagecustomizerapi.DiskSize(newDataSize).HumanReadable())
	}

	err = shell.NewExecBuilder("btrfs", "filesystem", "resize", strconv.FormatUint(newDataSize, 10), usrDir).
		LogLevel(logrus.DebugLevel, logrus.WarnLevel).
		ErrorStderrLines(1).
		Execute()
	if err != nil {
		return fmt.Errorf("%w: btrfs resize failed on /usr:\n%w", ErrAclGrowFilesystem, err)
	}

	return nil
}

// aclUsrVerityDataSize returns the data size the grown USR partition should be sealed at, and
// whether the image uses inline verity. Used to override the base-image verity metadata so verity
// is re-sealed to match the grown partition.
func aclUsrVerityDataSize(rawImageFile string) (uint64, bool, error) {
	loopback, err := safeloopback.NewLoopback(rawImageFile)
	if err != nil {
		return 0, false, err
	}
	defer loopback.Close()

	partitions, err := diskutils.GetDiskPartitions(loopback.DevicePath())
	if err != nil {
		return 0, false, err
	}

	usrPart, ok := findAclUsrPartition(partitions)
	if !ok {
		// Fall back to matching by label when nothing is mounted.
		byLabel := partitionsByLabel(partitions)
		usrPart, ok = byLabel[aclPartLabelUsrA]
		if !ok {
			return 0, false, fmt.Errorf("%w: could not find USR partition", ErrAclGrowFilesystem)
		}
	}

	dataSize, inlineVerity, err := aclUsrFilesystemSize(partitions, usrPart)
	if err != nil {
		return 0, false, err
	}

	err = loopback.CleanClose()
	if err != nil {
		return 0, false, err
	}

	return dataSize, inlineVerity, nil
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

// overrideAclUsrVerityMetadata rewrites the USR verity device's data size to match the grown /usr
// partition, so the subsequent re-seal covers the whole grown filesystem. The metadata read from
// the base image describes the original, smaller /usr.
//
// With inline verity the hash offset moves with the data and must be rewritten too. With a
// dedicated hash partition there is no offset, and it is left at zero so no hash-offset reaches the
// rebuilt UKI command line.
func overrideAclUsrVerityMetadata(rawImageFile string, verityMetadata []verityDeviceMetadata) error {
	dataSize, inlineVerity, err := aclUsrVerityDataSize(rawImageFile)
	if err != nil {
		return err
	}

	found := false
	for i := range verityMetadata {
		if verityMetadata[i].name == imagecustomizerapi.VerityUsrDeviceName {
			verityMetadata[i].formatSettings.dataSizeBytes = dataSize
			if inlineVerity {
				verityMetadata[i].formatSettings.hashOffsetBytes = dataSize
			}
			found = true
		}
	}

	if !found {
		return fmt.Errorf("%w: no /usr verity device found to re-seal after grow", ErrAclGrowFilesystem)
	}

	return nil
}

// aclUsrHashPartitionSize returns the partition size needed to hold the dm-verity hash tree for a
// /usr filesystem of the given size, rounded up to a whole MiB.
//
// The returned size is derived with the same calculation veritysetup uses (and includes the verity
// superblock), then aligned up to 1 MiB because partitions are MiB-aligned and a hash tree that
// overflows its partition produces an image that fails to boot rather than failing to build.
func aclUsrHashPartitionSize(usrPartitionSize uint64) (uint64, error) {
	dataBlockSize := uint64(imagecustomizerapi.DefaultVerityDataBlockSize)
	hashBlockSize := uint32(imagecustomizerapi.DefaultVerityHashBlockSize)

	if usrPartitionSize%dataBlockSize != 0 {
		return 0, fmt.Errorf("%w: /usr size %s is not a multiple of the verity data block size (%d)",
			ErrAclGrowUnexpectedLayout, imagecustomizerapi.DiskSize(usrPartitionSize).HumanReadable(),
			dataBlockSize)
	}

	hashSize, err := verityutils.CalculateHashSizeInBytes(usrPartitionSize/dataBlockSize, hashBlockSize,
		imagecustomizerapi.DefaultVerityHashAlgorithm)
	if err != nil {
		return 0, fmt.Errorf("%w: failed to compute /usr verity hash tree size:\n%w", ErrAclGrowFilesystem, err)
	}

	return mathutils.RoundUp(hashSize, uint64(diskutils.MiB)), nil
}

// aclUsrFilesystemSize returns the size the /usr filesystem should be grown to, and whether the
// image seals /usr with inline verity.
//
// With a dedicated hash partition the filesystem fills its partition. With inline verity the hash
// tree is written after the data inside the same partition, so the filesystem must stop short of
// the partition end.
func aclUsrFilesystemSize(partitions []diskutils.PartitionInfo, usrPart diskutils.PartitionInfo,
) (uint64, bool, error) {
	if aclUsrHasDedicatedHashPartition(partitions, usrPart) {
		return usrPart.SizeInBytes, false, nil
	}

	dataSize, err := imagecustomizerapi.CalculateInlineVerityDataSize(usrPart.SizeInBytes)
	if err != nil {
		return 0, true, fmt.Errorf("%w: failed to compute /usr inline verity data size:\n%w",
			ErrAclGrowFilesystem, err)
	}

	return dataSize, true, nil
}

// aclUsrHasDedicatedHashPartition reports whether the given /usr partition has a companion verity
// hash partition on the same disk.
func aclUsrHasDedicatedHashPartition(partitions []diskutils.PartitionInfo,
	usrPart diskutils.PartitionInfo,
) bool {
	hashLabel, ok := aclUsrHashPartLabels[usrPart.PartLabel]
	if !ok {
		// An unlabelled or unexpected /usr: fall back to detecting any hash partition on the disk.
		_, hasA := partitionsByLabel(partitions)[aclPartLabelHashA]
		return hasA
	}

	_, ok = partitionsByLabel(partitions)[hashLabel]
	return ok
}
