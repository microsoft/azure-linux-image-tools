package imagecustomizerlib

import (
	"archive/tar"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/randomization"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
)

type ImageBuildData struct {
	Source       string
	KnownInfo    outputPartitionMetadata
	Metadata     *FileSystem
	VeritySource string
}

func convertToCosi(buildDirAbs string, rawImageFile string, outputImageFile string,
	partUuidToFstabEntry map[string]diskutils.FstabEntry, verityMetadata []verityDeviceMetadata,
	osRelease string, osPackages []OsPackage, imageUuid [randomization.UuidSize]byte, imageUuidStr string,
	cosiBootMetadata *CosiBootloader,
) error {
	outputImageBase := strings.TrimSuffix(filepath.Base(outputImageFile), filepath.Ext(outputImageFile))
	outputDir := filepath.Join(buildDirAbs, "cosiimages")
	err := os.MkdirAll(outputDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create folder %s:\n%w", outputDir, err)
	}
	defer os.Remove(outputDir)

	imageLoopback, err := safeloopback.NewLoopback(rawImageFile)
	if err != nil {
		return err
	}
	defer imageLoopback.Close()

	partitionMetadataOutput, err := extractPartitions(imageLoopback.DevicePath(), outputDir, outputImageBase,
		"raw-zst", imageUuid)
	if err != nil {
		return err
	}
	for _, partition := range partitionMetadataOutput {
		defer os.Remove(path.Join(outputDir, partition.PartitionFilename))
	}

	err = buildCosiFile(outputDir, outputImageFile, partitionMetadataOutput, verityMetadata,
		partUuidToFstabEntry, imageUuidStr, osRelease, osPackages, cosiBootMetadata)
	if err != nil {
		return fmt.Errorf("failed to build COSI file:\n%w", err)
	}

	logger.Log.Infof("Successfully converted to COSI: %s", outputImageFile)

	err = imageLoopback.CleanClose()
	if err != nil {
		return err
	}

	return nil
}

func buildCosiFile(sourceDir string, outputFile string, partitions []outputPartitionMetadata,
	verityMetadata []verityDeviceMetadata, partUuidToFstabEntry map[string]diskutils.FstabEntry,
	imageUuidStr string, osRelease string, osPackages []OsPackage, cosiBootMetadata *CosiBootloader,
) error {
	// Pre-compute a map for quick lookup of partition metadata by UUID
	partUuidToMetadata := make(map[string]outputPartitionMetadata)
	for _, partition := range partitions {
		partUuidToMetadata[partition.PartUuid] = partition
	}

	// Pre-compute a set of verity hash UUIDs for quick lookup
	verityHashUuids := make(map[string]struct{})
	for _, verity := range verityMetadata {
		verityHashUuids[verity.hashPartUuid] = struct{}{}
	}

	imageData := []ImageBuildData{}

	for _, partition := range partitions {
		// Skip verity hash partitions as their metadata will be assigned to the corresponding data partitions
		if _, isVerityHash := verityHashUuids[partition.PartUuid]; isVerityHash {
			continue
		}

		// Skip partitions that are unmounted or have no filesystem type
		fstabEntry, hasMount := partUuidToFstabEntry[partition.PartUuid]
		if !hasMount || fstabEntry.Target == "" || partition.FileSystemType == "" {
			continue
		}

		metadataImage := FileSystem{
			Image: ImageFile{
				Path:             path.Join("images", partition.PartitionFilename),
				UncompressedSize: partition.UncompressedSize,
			},
			PartType:   partition.PartitionTypeUuid,
			MountPoint: partUuidToFstabEntry[partition.PartUuid].Target,
			FsType:     partition.FileSystemType,
			FsUuid:     partition.Uuid,
		}

		imageDataEntry := ImageBuildData{
			Source:    path.Join(sourceDir, partition.PartitionFilename),
			Metadata:  &metadataImage,
			KnownInfo: partition,
		}

		// Add Verity metadata if the partition has a matching entry in verityMetadata
		for _, verity := range verityMetadata {
			if partition.PartUuid == verity.dataPartUuid {
				hashPartition, exists := partUuidToMetadata[verity.hashPartUuid]
				if !exists {
					return fmt.Errorf("missing metadata for hash partition UUID:\n%s", verity.hashPartUuid)
				}

				metadataImage.Verity = &VerityConfig{
					Roothash: verity.rootHash,
					Image: ImageFile{
						Path:             path.Join("images", hashPartition.PartitionFilename),
						UncompressedSize: hashPartition.UncompressedSize,
					},
				}

				veritySourcePath := path.Join(sourceDir, hashPartition.PartitionFilename)
				imageDataEntry.VeritySource = veritySourcePath
				break
			}
		}

		imageData = append(imageData, imageDataEntry)
	}

	// Populate metadata for each image
	for i := range imageData {
		err := populateMetadata(&imageData[i])
		if err != nil {
			return fmt.Errorf("failed to populate metadata for %s:\n%w", imageData[i].Source, err)
		}

		logger.Log.Infof("Populated metadata for image %s", imageData[i].Source)
	}

	metadata := MetadataJson{
		Version:    "1.1",
		OsArch:     getArchitectureForCosi(),
		Id:         imageUuidStr,
		Images:     make([]FileSystem, len(imageData)),
		OsRelease:  osRelease,
		OsPackages: osPackages,
		Bootloader: *cosiBootMetadata,
	}

	// Copy updated metadata
	for i, data := range imageData {
		metadata.Images[i] = *data.Metadata
	}

	// Marshal metadata.json
	metadataJson, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata:\n%w", err)
	}

	// Create COSI file
	cosiFile, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create COSI file:\n%w", err)
	}
	defer cosiFile.Close()

	tw := tar.NewWriter(cosiFile)
	defer tw.Close()

	tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "metadata.json",
		Size:     int64(len(metadataJson)),
		Mode:     0o400,
		Format:   tar.FormatPAX,
	})
	tw.Write(metadataJson)

	for _, data := range imageData {
		if err := addToCosi(data, tw); err != nil {
			return fmt.Errorf("failed to add %s to COSI:\n%w", data.Source, err)
		}
	}

	logger.Log.Infof("Finished building COSI: %s", outputFile)
	return nil
}

func addToCosi(data ImageBuildData, tw *tar.Writer) error {
	err := addFileToCosi(tw, data.Source, data.Metadata.Image)
	if err != nil {
		return fmt.Errorf("failed to add image file to COSI:\n%w", err)
	}

	if data.VeritySource != "" && data.Metadata.Verity != nil {
		err := addFileToCosi(tw, data.VeritySource, data.Metadata.Verity.Image)
		if err != nil {
			return fmt.Errorf("failed to add verity file to COSI:\n%w", err)
		}
	}

	return nil
}

func addFileToCosi(tw *tar.Writer, source string, image ImageFile) error {
	file, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open file :\n%w", err)
	}
	defer file.Close()

	err = tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     image.Path,
		Size:     int64(image.CompressedSize),
		Mode:     0o400,
		Format:   tar.FormatPAX,
	})
	if err != nil {
		return fmt.Errorf("failed to write tar header for file '%s':\n%w", image.Path, err)
	}

	_, err = io.Copy(tw, file)
	if err != nil {
		return fmt.Errorf("failed to write image '%s' to COSI:\n%w", image.Path, err)
	}

	return nil
}

func sha384sum(path string) (string, error) {
	sha384 := sha512.New384()
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if _, err := io.Copy(sha384, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha384.Sum(nil)), nil
}

func populateImageFile(source string, imageFile *ImageFile) error {
	stat, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("failed to stat %s:\n%w", source, err)
	}
	if stat.IsDir() {
		return fmt.Errorf("%s is a directory", source)
	}
	imageFile.CompressedSize = uint64(stat.Size())

	sha384, err := sha384sum(source)
	if err != nil {
		return fmt.Errorf("failed to calculate sha384 of %s:\n%w", source, err)
	}
	imageFile.Sha384 = sha384

	return nil
}

// Enriches the image metadata with size and checksum
func populateMetadata(data *ImageBuildData) error {
	if err := populateImageFile(data.Source, &data.Metadata.Image); err != nil {
		return fmt.Errorf("failed to populate metadata:\n%w", err)
	}

	if err := populateVerityMetadata(data.VeritySource, data.Metadata.Verity); err != nil {
		return fmt.Errorf("failed to populate verity metadata:\n%w", err)
	}

	return nil
}

func populateVerityMetadata(source string, verity *VerityConfig) error {
	if source == "" && verity == nil {
		return nil
	}

	if source == "" || verity == nil {
		return fmt.Errorf("verity source and verity metadata must be both defined or both undefined")
	}

	if err := populateImageFile(source, &verity.Image); err != nil {
		return fmt.Errorf("failed to populate verity image metadata:\n%w", err)
	}

	return nil
}

func getArchitectureForCosi() string {
	if runtime.GOARCH == "amd64" {
		return "x86_64"
	}
	return runtime.GOARCH
}

func getAllPackagesFromChroot(imageConnection *imageconnection.ImageConnection) ([]OsPackage, error) {
	if !isPackageInstalled(imageConnection.Chroot(), "rpm") {
		return nil, fmt.Errorf("'rpm' is not installed in the image to enable package listing for COSI output. You may add it via the 'packages:' section in your configuration YAML")
	}

	var out string
	err := imageConnection.Chroot().UnsafeRun(func() error {
		var err error
		out, _, err = shell.Execute(
			"rpm", "-qa", "--queryformat", "%{NAME} %{VERSION} %{RELEASE} %{ARCH}\n",
		)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get RPM output from chroot:\n%w", err)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	var packages []OsPackage
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) != 4 {
			return nil, fmt.Errorf("malformed RPM line encountered while parsing installed RPMs for COSI: %q", line)
		}
		packages = append(packages, OsPackage{
			Name:    parts[0],
			Version: parts[1],
			Release: parts[2],
			Arch:    parts[3],
		})
	}

	return packages, nil
}

func extractCosiBootMetadata(buildDirAbs string, imageConnection *imageconnection.ImageConnection) (*CosiBootloader, error) {
	bootloaderType, err := DetectBootloaderType(imageConnection.Chroot())
	if err != nil {
		return nil, fmt.Errorf("failed to detect bootloader type: %w", err)
	}

	chrootDir := imageConnection.Chroot().RootDir()

	switch bootloaderType {
	case BootloaderTypeSystemdBoot:
		// Handles UKI + config and config-only
		entries, err := extractSystemdBootEntriesIfPresent(chrootDir)
		if err != nil {
			return nil, err
		}
		if len(entries) > 0 {
			return &CosiBootloader{
				Type:        BootloaderTypeSystemdBoot,
				SystemdBoot: &SystemDBoot{Entries: entries},
			}, nil
		}

		// Handles UKI standalone .efi images
		entries, err = extractUkiEntriesIfPresent(chrootDir, buildDirAbs)
		if err != nil {
			return nil, err
		}
		if len(entries) > 0 {
			return &CosiBootloader{
				Type:        BootloaderTypeSystemdBoot,
				SystemdBoot: &SystemDBoot{Entries: entries},
			}, nil
		}

	case BootloaderTypeGrub:
		return &CosiBootloader{
			Type:        BootloaderTypeGrub,
			SystemdBoot: nil,
		}, nil
	}

	return nil, fmt.Errorf("no supported bootloader configuration found")
}

func extractUkiEntriesIfPresent(chrootDir, buildDir string) ([]SystemDBootEntry, error) {
	espDir := filepath.Join(chrootDir, "boot/efi")
	cmdlines, err := extractKernelCmdlineFromUkiEfis(espDir, buildDir)
	if err != nil {
		return nil, fmt.Errorf("failed to extract kernel cmdline from UKI .efi files:\n%w", err)
	}

	var entries []SystemDBootEntry
	for kernelName, cmdline := range cmdlines {
		efiPath := filepath.Join("/boot/efi/EFI/Linux", fmt.Sprintf("%s.efi", kernelName))
		kernelVersion, err := getKernelVersion(kernelName)
		if err != nil {
			return nil, fmt.Errorf("invalid kernel name in UKI file (%s):\n%w", kernelName, err)
		}
		entries = append(entries, SystemDBootEntry{
			Type:    SystemDBootEntryTypeUKIStandalone,
			Path:    efiPath,
			Kernel:  kernelVersion,
			Cmdline: strings.TrimRight(cmdline, "\n"),
		})
	}
	return entries, nil
}

func extractSystemdBootEntriesIfPresent(chrootDir string) ([]SystemDBootEntry, error) {
	loaderEntryDir := filepath.Join(chrootDir, "boot", "loader", "entries")
	exists, err := file.DirExists(loaderEntryDir)
	if err != nil {
		return nil, fmt.Errorf("failed while checking if systemd-boot entry dir '%s' exists:\n%w", loaderEntryDir, err)
	}
	if !exists {
		return nil, nil
	}

	entries, err := extractSystemdBootEntries(loaderEntryDir)
	if err != nil {
		return nil, fmt.Errorf("failed to extract systemd-boot entries:\n%w", err)
	}

	return entries, nil
}

func extractSystemdBootEntries(entryDir string) ([]SystemDBootEntry, error) {
	files, err := os.ReadDir(entryDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s:\n%w", entryDir, err)
	}

	var entries []SystemDBootEntry

	for _, file := range files {
		entry, err := parseSystemdBootEntryFromFile(entryDir, file)
		if err != nil {
			return nil, err
		}
		if entry != nil {
			entries = append(entries, *entry)
		}
	}

	return entries, nil
}

func parseSystemdBootEntryFromFile(entryDir string, file fs.DirEntry) (*SystemDBootEntry, error) {
	if file.IsDir() || !strings.HasSuffix(file.Name(), ".conf") {
		return nil, nil
	}

	absPath := filepath.Join(entryDir, file.Name())
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", absPath, err)
	}

	entry := &SystemDBootEntry{
		Path: filepath.Join("/boot/loader/entries", file.Name()),
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		// Handle line endings
		line = strings.TrimRight(line, "\r\n")

		// Ignore blank lines and comment lines (even with leading spaces/tabs)
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Split key and value on first space/tab
		idx := strings.IndexAny(line, " \t")
		if idx == -1 {
			continue // No key-value separator
		}

		key := strings.TrimSpace(line[:idx])

		value := strings.TrimLeft(line[idx:], " \t") // Remove leading separators
		value = strings.TrimRight(value, " \t")      // Remove trailing whitespace

		// Remove surrounding quotes from value, if any
		if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") && len(value) >= 2 {
			value = value[1 : len(value)-1]
		}

		switch key {
		case "options":
			if entry.Cmdline != "" {
				entry.Cmdline += " "
			}
			entry.Cmdline += value

		case "linux":
			if kernelVer, err := getKernelVersion(filepath.Base(value)); err == nil {
				entry.Kernel = kernelVer
			}

		case "uki":
			if kernelVer, err := getKernelVersion(filepath.Base(value)); err == nil {
				entry.Kernel = kernelVer
			}
			entry.Type = SystemDBootEntryTypeUKIConfig
		}
	}

	// Determine fallback type
	if entry.Type == "" {
		if strings.HasSuffix(entry.Kernel, ".efi") {
			entry.Type = SystemDBootEntryTypeUKIConfig
		} else {
			entry.Type = SystemDBootEntryTypeConfig
		}
	}

	return entry, nil
}

func DetectBootloaderType(imageChroot safechroot.ChrootInterface) (BootloaderType, error) {
	if isPackageInstalled(imageChroot, "grub2-efi-binary") {
		return BootloaderTypeGrub, nil
	}
	if isPackageInstalled(imageChroot, "systemd-boot") {
		return BootloaderTypeSystemdBoot, nil
	}
	return "", fmt.Errorf("unknown bootloader: neither grub2-efi-binary nor systemd-boot found")
}
