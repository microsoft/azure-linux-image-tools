// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/stretchr/testify/assert"
)

func TestConvertImageRawToVhd(t *testing.T) {
	for _, baseImageInfo := range checkSkipForCustomizeDefaultImages(t) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testConvertImageRawToVhd(t, baseImageInfo)
		})
	}
}

func testConvertImageRawToVhd(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTempDir := filepath.Join(tmpDir, fmt.Sprintf("TestConvertImageRawToVhd_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outputImageFile := filepath.Join(testTempDir, "output.vhd")

	options := ConvertImageOptions{
		BuildDir:          buildDir,
		InputImageFile:    baseImage,
		OutputImageFile:   outputImageFile,
		OutputImageFormat: imagecustomizerapi.ImageFormatTypeVhd,
	}

	err := ConvertImage(t.Context(), options)
	if !assert.NoError(t, err) {
		return
	}

	// Verify output file type
	checkFileType(t, outputImageFile, "vhd")
}

func TestConvertImageRawToQcow2(t *testing.T) {
	for _, baseImageInfo := range checkSkipForCustomizeDefaultImages(t) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testConvertImageRawToQcow2(t, baseImageInfo)
		})
	}
}

func testConvertImageRawToQcow2(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTempDir := filepath.Join(tmpDir, fmt.Sprintf("TestConvertImageRawToQcow2_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outputImageFile := filepath.Join(testTempDir, "output.qcow2")

	options := ConvertImageOptions{
		BuildDir:          buildDir,
		InputImageFile:    baseImage,
		OutputImageFile:   outputImageFile,
		OutputImageFormat: imagecustomizerapi.ImageFormatTypeQcow2,
	}

	err := ConvertImage(t.Context(), options)
	if !assert.NoError(t, err) {
		return
	}

	// Verify output file type
	checkFileType(t, outputImageFile, "qcow2")
}

func TestConvertImageRawToCosi(t *testing.T) {
	for _, baseImageInfo := range checkSkipForCustomizeDefaultImages(t) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testConvertImageRawToCosi(t, baseImageInfo)
		})
	}
}

func testConvertImageRawToCosi(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	if baseImageInfo.Distro == baseImageDistroAzureLinux && baseImageInfo.Version == baseImageVersionAzl2 {
		t.Skip("'systemd-boot' is not available on Azure Linux 2.0")
	}

	ukifyExists, err := file.CommandExists("ukify")
	assert.NoError(t, err)
	if !ukifyExists {
		t.Skip("The 'ukify' command is not available")
	}

	if runtime.GOARCH == "arm64" {
		t.Skip("systemd-boot not available on AZL3 ARM64 yet")
	}

	testTempDir := filepath.Join(tmpDir, fmt.Sprintf("TestConvertImageRawToCosi_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outputImageFile := filepath.Join(testTempDir, "output.cosi")

	// First, we need a customized image with verity enabled
	customizedImage := filepath.Join(testTempDir, "customized.raw")
	configFile := filepath.Join(testDir, "verity-config.yaml")

	err = CustomizeImageWithConfigFileOptions(t.Context(), configFile, ImageCustomizerOptions{
		BuildDir:             buildDir,
		InputImageFile:       baseImage,
		OutputImageFile:      customizedImage,
		OutputImageFormat:    "raw",
		UseBaseImageRpmRepos: true,
		PreviewFeatures:      baseImageInfo.PreviewFeatures,
	})
	if baseImageInfo.Distro == baseImageDistroUbuntu {
		// TODO: Remove this check once Ubuntu supports bootloader hard-reset.
		assert.ErrorContains(t, err, "bootloader hard-reset is not supported for Ubuntu images")
		return
	}
	if !assert.NoError(t, err) {
		return
	}

	// Now convert to COSI
	buildDir2 := filepath.Join(testTempDir, "build2")
	options := ConvertImageOptions{
		BuildDir:          buildDir2,
		InputImageFile:    customizedImage,
		OutputImageFile:   outputImageFile,
		OutputImageFormat: imagecustomizerapi.ImageFormatTypeCosi,
	}

	err = ConvertImage(t.Context(), options)
	if !assert.NoError(t, err) {
		return
	}

	// Verify output file exists
	exists, err := file.PathExists(outputImageFile)
	assert.NoError(t, err)
	assert.True(t, exists, "Expected output COSI file to exist")

	// Verify COSI file is not empty and has reasonable size
	cosiStat, err := os.Stat(outputImageFile)
	assert.NoError(t, err)
	assert.Greater(t, cosiStat.Size(), int64(100*diskutils.MiB), "COSI file should be at least 100 MiB")
}

func TestConvertImageRawToCosiWithCompression(t *testing.T) {
	for _, baseImageInfo := range checkSkipForCustomizeDefaultImages(t) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testConvertImageRawToCosiWithCompression(t, baseImageInfo)
		})
	}
}

func testConvertImageRawToCosiWithCompression(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	if baseImageInfo.Distro == baseImageDistroAzureLinux && baseImageInfo.Version == baseImageVersionAzl2 {
		t.Skip("'systemd-boot' is not available on Azure Linux 2.0")
	}

	ukifyExists, err := file.CommandExists("ukify")
	assert.NoError(t, err)
	if !ukifyExists {
		t.Skip("The 'ukify' command is not available")
	}

	if runtime.GOARCH == "arm64" {
		t.Skip("systemd-boot not available on AZL3 ARM64 yet")
	}

	testTempDir := filepath.Join(tmpDir,
		fmt.Sprintf("TestConvertImageRawToCosiWithCompression_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outputImageFile := filepath.Join(testTempDir, "output.cosi")

	// First, we need a customized image with verity enabled
	customizedImage := filepath.Join(testTempDir, "customized.raw")
	configFile := filepath.Join(testDir, "verity-config.yaml")

	err = CustomizeImageWithConfigFileOptions(t.Context(), configFile, ImageCustomizerOptions{
		BuildDir:             buildDir,
		InputImageFile:       baseImage,
		OutputImageFile:      customizedImage,
		OutputImageFormat:    "raw",
		UseBaseImageRpmRepos: true,
		PreviewFeatures:      baseImageInfo.PreviewFeatures,
	})
	if baseImageInfo.Distro == baseImageDistroUbuntu {
		// TODO: Remove this check once Ubuntu supports bootloader hard-reset.
		assert.ErrorContains(t, err, "bootloader hard-reset is not supported for Ubuntu images")
		return
	}
	if !assert.NoError(t, err) {
		return
	}

	// Now convert to COSI with compression level 10
	buildDir2 := filepath.Join(testTempDir, "build2")
	compressionLevel := 10
	options := ConvertImageOptions{
		BuildDir:             buildDir2,
		InputImageFile:       customizedImage,
		OutputImageFile:      outputImageFile,
		OutputImageFormat:    imagecustomizerapi.ImageFormatTypeCosi,
		CosiCompressionLevel: &compressionLevel,
	}

	err = ConvertImage(t.Context(), options)
	if !assert.NoError(t, err) {
		return
	}

	// Verify output file exists
	exists, err := file.PathExists(outputImageFile)
	assert.NoError(t, err)
	assert.True(t, exists, "Expected output COSI file to exist")

	// Verify COSI file is not empty
	cosiStat, err := os.Stat(outputImageFile)
	assert.NoError(t, err)
	assert.Greater(t, cosiStat.Size(), int64(100*diskutils.MiB), "COSI file should be at least 100 MiB")
}

func TestConvertImageRawToBareMetalImage(t *testing.T) {
	for _, baseImageInfo := range checkSkipForCustomizeDefaultImages(t) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testConvertImageRawToBareMetalImage(t, baseImageInfo)
		})
	}
}

func testConvertImageRawToBareMetalImage(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	if baseImageInfo.Distro == baseImageDistroAzureLinux && baseImageInfo.Version == baseImageVersionAzl2 {
		t.Skip("'systemd-boot' is not available on Azure Linux 2.0")
	}

	ukifyExists, err := file.CommandExists("ukify")
	assert.NoError(t, err)
	if !ukifyExists {
		t.Skip("The 'ukify' command is not available")
	}

	if runtime.GOARCH == "arm64" {
		t.Skip("systemd-boot not available on AZL3 ARM64 yet")
	}

	testTempDir := filepath.Join(tmpDir,
		fmt.Sprintf("TestConvertImageRawToBareMetalImage_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outputImageFile := filepath.Join(testTempDir, "output.vhd")

	// First, we need a customized image with verity enabled
	customizedImage := filepath.Join(testTempDir, "customized.raw")
	configFile := filepath.Join(testDir, "verity-config.yaml")

	err = CustomizeImageWithConfigFileOptions(t.Context(), configFile, ImageCustomizerOptions{
		BuildDir:             buildDir,
		InputImageFile:       baseImage,
		OutputImageFile:      customizedImage,
		OutputImageFormat:    "raw",
		UseBaseImageRpmRepos: true,
		PreviewFeatures:      baseImageInfo.PreviewFeatures,
	})
	if baseImageInfo.Distro == baseImageDistroUbuntu {
		// TODO: Remove this check once Ubuntu supports bootloader hard-reset.
		assert.ErrorContains(t, err, "bootloader hard-reset is not supported for Ubuntu images")
		return
	}
	if !assert.NoError(t, err) {
		return
	}

	// Now convert to bare-metal-image
	buildDir2 := filepath.Join(testTempDir, "build2")
	options := ConvertImageOptions{
		BuildDir:          buildDir2,
		InputImageFile:    customizedImage,
		OutputImageFile:   outputImageFile,
		OutputImageFormat: imagecustomizerapi.ImageFormatTypeBareMetalImage,
	}

	err = ConvertImage(t.Context(), options)
	if !assert.NoError(t, err) {
		return
	}

	// Verify output file exists
	exists, err := file.PathExists(outputImageFile)
	assert.NoError(t, err)
	assert.True(t, exists, "Expected output bare-metal-image file to exist")

	// Verify file is not empty
	stat, err := os.Stat(outputImageFile)
	assert.NoError(t, err)
	assert.Greater(t, stat.Size(), int64(100*diskutils.MiB), "bare-metal-image should be at least 100 MiB")
}

func TestConvertImageInvalidInputFile(t *testing.T) {
	testTempDir := filepath.Join(tmpDir, "TestConvertImageInvalidInputFile")
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outputImageFile := filepath.Join(testTempDir, "output.vhdx")

	options := ConvertImageOptions{
		BuildDir:          buildDir,
		InputImageFile:    "/nonexistent/image.raw",
		OutputImageFile:   outputImageFile,
		OutputImageFormat: imagecustomizerapi.ImageFormatTypeVhdx,
	}

	err := ConvertImage(t.Context(), options)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidInputImageFileArg)
}

func TestConvertImageCosiCompressionInvalidFormat(t *testing.T) {
	baseImage, _ := checkSkipForCustomizeDefaultAzureLinuxImage(t)

	testTempDir := filepath.Join(tmpDir, "TestConvertImageCosiCompressionInvalidFormat")
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outputImageFile := filepath.Join(testTempDir, "output.vhdx")

	compressionLevel := 10
	options := ConvertImageOptions{
		BuildDir:             buildDir,
		InputImageFile:       baseImage,
		OutputImageFile:      outputImageFile,
		OutputImageFormat:    imagecustomizerapi.ImageFormatTypeVhdx,
		CosiCompressionLevel: &compressionLevel,
	}

	err := ConvertImage(t.Context(), options)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "COSI compression level can only be specified for COSI or bare-metal-image output formats")
}

func TestConvertImageAutoDetectFormat(t *testing.T) {
	for _, baseImageInfo := range checkSkipForCustomizeDefaultImages(t) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testConvertImageAutoDetectFormat(t, baseImageInfo)
		})
	}
}

func testConvertImageAutoDetectFormat(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTempDir := filepath.Join(tmpDir, fmt.Sprintf("TestConvertImageAutoDetectFormat_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTempDir)

	err := os.MkdirAll(testTempDir, os.ModePerm)
	if !assert.NoError(t, err) {
		return
	}

	// First convert base image to VHDX
	intermediateVhdx := filepath.Join(testTempDir, "intermediate.vhdx")
	err = ConvertImageFile(baseImage, intermediateVhdx, imagecustomizerapi.ImageFormatTypeVhdx)
	if !assert.NoError(t, err) {
		return
	}

	// Now convert back to RAW without specifying output format (should auto-detect VHDX input)
	buildDir2 := filepath.Join(testTempDir, "build2")
	outputRaw := filepath.Join(testTempDir, "output.raw")

	options := ConvertImageOptions{
		BuildDir:          buildDir2,
		InputImageFile:    intermediateVhdx,
		OutputImageFile:   outputRaw,
		OutputImageFormat: imagecustomizerapi.ImageFormatTypeRaw,
	}

	err = ConvertImage(t.Context(), options)
	if !assert.NoError(t, err) {
		return
	}

	// Verify output file type
	checkFileType(t, outputRaw, "raw")
}

func TestConvertImageRoundTrip(t *testing.T) {
	for _, baseImageInfo := range checkSkipForCustomizeDefaultImages(t) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testConvertImageRoundTrip(t, baseImageInfo)
		})
	}
}

func testConvertImageRoundTrip(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTempDir := filepath.Join(tmpDir, fmt.Sprintf("TestConvertImageRoundTrip_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTempDir)

	// RAW → VHDX
	buildDir1 := filepath.Join(testTempDir, "build1")
	vhdxImage := filepath.Join(testTempDir, "step1.vhdx")
	options1 := ConvertImageOptions{
		BuildDir:          buildDir1,
		InputImageFile:    baseImage,
		OutputImageFile:   vhdxImage,
		OutputImageFormat: imagecustomizerapi.ImageFormatTypeVhdx,
	}
	err := ConvertImage(t.Context(), options1)
	if !assert.NoError(t, err) {
		return
	}

	checkFileType(t, vhdxImage, "vhdx")

	// VHDX → QCOW2
	buildDir2 := filepath.Join(testTempDir, "build2")
	qcow2Image := filepath.Join(testTempDir, "step2.qcow2")
	options2 := ConvertImageOptions{
		BuildDir:          buildDir2,
		InputImageFile:    vhdxImage,
		OutputImageFile:   qcow2Image,
		OutputImageFormat: imagecustomizerapi.ImageFormatTypeQcow2,
	}
	err = ConvertImage(t.Context(), options2)
	if !assert.NoError(t, err) {
		return
	}

	checkFileType(t, qcow2Image, "qcow2")

	// QCOW2 → VHD
	buildDir3 := filepath.Join(testTempDir, "build3")
	vhdImage := filepath.Join(testTempDir, "step3.vhd")
	options3 := ConvertImageOptions{
		BuildDir:          buildDir3,
		InputImageFile:    qcow2Image,
		OutputImageFile:   vhdImage,
		OutputImageFormat: imagecustomizerapi.ImageFormatTypeVhd,
	}
	err = ConvertImage(t.Context(), options3)
	if !assert.NoError(t, err) {
		return
	}

	checkFileType(t, vhdImage, "vhd")

	// VHD → RAW
	buildDir4 := filepath.Join(testTempDir, "build4")
	rawImage := filepath.Join(testTempDir, "step4.raw")
	options4 := ConvertImageOptions{
		BuildDir:          buildDir4,
		InputImageFile:    vhdImage,
		OutputImageFile:   rawImage,
		OutputImageFormat: imagecustomizerapi.ImageFormatTypeRaw,
	}
	err = ConvertImage(t.Context(), options4)
	if !assert.NoError(t, err) {
		return
	}

	checkFileType(t, rawImage, "raw")

	// Verify final RAW image is bootable by connecting to it
	imageConnection, err := testutils.ConnectToImage(buildDir4, rawImage, false, nil)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()
}
