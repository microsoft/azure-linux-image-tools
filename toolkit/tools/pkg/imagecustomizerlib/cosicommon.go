package imagecustomizerlib

import (
	"archive/tar"
	"crypto/sha512"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safeloopback"
)

type ImageBuildData struct {
	Source    string
	KnownInfo outputPartitionMetadata
	Metadata  *Image
}

func convertToCosi(ic *ImageCustomizerParameters) error {
	logger.Log.Infof("Extracting partition files")
	outputDir := filepath.Join(ic.buildDir, "cosiimages")
	err := os.MkdirAll(outputDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create folder %s:\n%w", outputDir, err)
	}

	imageLoopback, err := safeloopback.NewLoopback(ic.rawImageFile)
	if err != nil {
		return err
	}
	defer imageLoopback.Close()

	partitionMetadataOutput, err := extractPartitions(imageLoopback.DevicePath(), outputDir, ic.outputImageBase, "raw-zst", ic.imageUuid)
	if err != nil {
		return err
	}

	err = buildCosiFile(outputDir, ic.outputImageFile, partitionMetadataOutput, ic.imageUuidStr)
	if err != nil {
		return fmt.Errorf("failed to build COSI:\n%w", err)
	}

	logger.Log.Infof("Successfully converted to COSI: %s", ic.outputImageFile)

	err = imageLoopback.CleanClose()
	if err != nil {
		return err
	}

	return nil
}

func buildCosiFile(sourceDir string, outputFile string, expectedImages []outputPartitionMetadata, imageUuidStr string) error {
	metadata := MetadataJson{
		Version: "1.0",
		OsArch:  runtime.GOARCH,
		Id:      imageUuidStr,
		Images:  make([]Image, len(expectedImages)),
	}

	if len(expectedImages) == 0 {
		return fmt.Errorf("no images to build")
	}

	// Create an interim metadata struct to combine the known data with the metadata
	imageData := make([]ImageBuildData, len(expectedImages))
	for i, image := range expectedImages {
		metadata := &metadata.Images[i]
		imageData[i] = ImageBuildData{
			Source:    path.Join(sourceDir, image.PartitionFilename),
			Metadata:  metadata,
			KnownInfo: image,
		}

		metadata.Image.Path = path.Join("images", image.PartitionFilename)
		metadata.PartType = image.PartitionTypeUuid
		metadata.MountPoint = image.Mountpoint
		metadata.FsType = image.FileSystemType
		metadata.FsUuid = image.Uuid
		metadata.UncompressedSize = image.UncompressedSize
	}

	// Populate metadata for each image
	for _, data := range imageData {
		logger.Log.Infof("Processing image %s", data.Source)
		err := populateMetadata(data)
		if err != nil {
			return fmt.Errorf("failed to populate metadata for %s:\n%w", data.Source, err)
		}

		logger.Log.Infof("Populated metadata for image %s", data.Source)
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
	imageFile, err := os.Open(data.Source)
	if err != nil {
		return fmt.Errorf("failed to open image file:\n%w", err)
	}
	defer imageFile.Close()

	err = tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     data.Metadata.Image.Path,
		Size:     int64(data.Metadata.Image.CompressedSize),
		Mode:     0o400,
		Format:   tar.FormatPAX,
	})
	if err != nil {
		return fmt.Errorf("failed to write tar header:\n%w", err)
	}

	_, err = io.Copy(tw, imageFile)
	if err != nil {
		return fmt.Errorf("failed to write image to COSI:\n%w", err)
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

func populateMetadata(data ImageBuildData) error {
	stat, err := os.Stat(data.Source)
	if err != nil {
		return fmt.Errorf("filed to stat %s:\n%w", data.Source, err)
	}
	if stat.IsDir() {
		return fmt.Errorf("%s is a directory", data.Source)
	}
	data.Metadata.Image.CompressedSize = uint64(stat.Size())

	// Calculate the sha384 of the image
	sha384, err := sha384sum(data.Source)
	if err != nil {
		return fmt.Errorf("failed to calculate sha384 of %s:\n%w", data.Source, err)
	}
	data.Metadata.Image.Sha384 = sha384
	return nil
}
