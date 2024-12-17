package imagecustomizerlib

import (
	"archive/tar"
	"crypto/sha512"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

type ImageBuildData struct {
	Source    string
	KnownInfo ExpectedImage
	Metadata  *Image
}

type ExpectedImage struct {
	Name                string
	PartType            PartitionType
	MountPoint          string
	FsType              string
	FsUuid              string
	OsReleasePath       *string
	GrubCfgPath         *string
	ContainsRpmDatabase bool
}

func (ex ExpectedImage) ShouldMount() bool {
	return ex.OsReleasePath != nil || ex.GrubCfgPath != nil || ex.ContainsRpmDatabase
}

type ExtractedImageData struct {
	OsRelease string
	GrubCfg   string
}

func buildCosiFile(sourceDir string, outputFile string, expectedImages []ExpectedImage) error {
	metadata := MetadataJson{
		Version: "1.0",
		OsArch:  "x86_64",
		Id:      uuid.New().String(),
		Images:  make([]Image, len(expectedImages)),
	}

	if len(expectedImages) == 0 {
		return errors.New("no images to build")
	}

	// Create an interim metadata struct to combine the known data with the metadata
	imageData := make([]ImageBuildData, len(expectedImages))
	for i, image := range expectedImages {
		metadata := &metadata.Images[i]
		imageData[i] = ImageBuildData{
			Source:    path.Join(sourceDir, image.Name),
			Metadata:  metadata,
			KnownInfo: image,
		}

		metadata.Image.Path = path.Join("images", image.Name)
		metadata.PartType = image.PartType
		metadata.MountPoint = image.MountPoint
		metadata.FsType = image.FsType
		metadata.FsUuid = image.FsUuid
	}

	// Populate metadata for each image
	for _, data := range imageData {
		log.WithField("image", data.Source).Info("Processing image...")
		extracted, err := data.populateMetadata()
		if err != nil {
			return fmt.Errorf("failed to populate metadata for %s: %w", data.Source, err)
		}

		log.WithField("image", data.Source).Info("Populated metadata for image.")

		metadata.OsRelease = extracted.OsRelease
	}

	// Marshal metadata.json
	metadataJson, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Create COSI file
	cosiFile, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create COSI file: %w", err)
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
		if err := data.addToCosi(tw); err != nil {
			return fmt.Errorf("failed to add %s to COSI: %w", data.Source, err)
		}
	}

	log.Infof("Finished building COSI: %s", outputFile)
	return nil
}

func (data *ImageBuildData) addToCosi(tw *tar.Writer) error {
	imageFile, err := os.Open(data.Source)
	if err != nil {
		return fmt.Errorf("failed to open image file: %w", err)
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
		return fmt.Errorf("failed to write tar header: %w", err)
	}

	_, err = io.Copy(tw, imageFile)
	if err != nil {
		return fmt.Errorf("failed to write image to COSI: %w", err)
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

func (data *ImageBuildData) populateMetadata() (*ExtractedImageData, error) {
	stat, err := os.Stat(data.Source)
	if err != nil {
		return nil, fmt.Errorf("filed to stat %s: %w", data.Source, err)
	}
	if stat.IsDir() {
		return nil, fmt.Errorf("%s is a directory", data.Source)
	}
	data.Metadata.Image.CompressedSize = uint64(stat.Size())

	// Calculate the sha384 of the image
	sha384, err := sha384sum(data.Source)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate sha384 of %s: %w", data.Source, err)
	}
	data.Metadata.Image.Sha384 = sha384

	tmpFile, err := os.Open(data.Source)
	if err != nil {
		return nil, fmt.Errorf("failed to open uncompressed file %s: %w", data.Source, err)
	}

	defer tmpFile.Close()

	stat, err = tmpFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat decompressed image: %w", err)
	}

	data.Metadata.Image.UncompressedSize = uint64(stat.Size())
	var extractedData ExtractedImageData

	// If this image doesn't need to be mounted, we're done
	if !data.KnownInfo.ShouldMount() {
		return &extractedData, nil
	}

	return &extractedData, nil
}
