package imagecustomizerlib

import (
	"archive/tar"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
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
	osRelease string, osPackages []OsPackage, imageUuid [UuidSize]byte, imageUuidStr string,
) error {
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

	partitionMetadataOutput, err := extractPartitions(imageLoopback.DevicePath(), outputDir, "partition",
		"raw-zst", imageUuid)
	if err != nil {
		return err
	}
	for _, partition := range partitionMetadataOutput {
		defer os.Remove(path.Join(outputDir, partition.PartitionFilename))
	}

	err = buildCosiFile(outputDir, outputImageFile, partitionMetadataOutput, verityMetadata,
		partUuidToFstabEntry, imageUuidStr, osRelease, osPackages)
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
	imageUuidStr string, osRelease string, osPackages []OsPackage,
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
		Version:    "1.0",
		OsArch:     getArchitectureForCosi(),
		Id:         imageUuidStr,
		Images:     make([]FileSystem, len(imageData)),
		OsRelease:  osRelease,
		OsPackages: osPackages,
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

func getAllPackagesFromChroot(imageConnection *ImageConnection) ([]OsPackage, error) {
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
