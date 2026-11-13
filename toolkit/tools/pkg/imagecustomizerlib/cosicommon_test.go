// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/cosiapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type cosiBootMetadataTestDistro struct {
	DistroHandler
	bootloaderType cosiapi.BootloaderType
}

func (d *cosiBootMetadataTestDistro) DetectBootloaderType(_ safechroot.ChrootInterface,
	_ *safechroot.Chroot,
) (cosiapi.BootloaderType, error) {
	return d.bootloaderType, nil
}

func (*cosiBootMetadataTestDistro) GetEspDir() string {
	return "boot/efi"
}

func TestDetectBootloaderTypePackages(t *testing.T) {
	for _, test := range []struct {
		name        string
		packages    []string
		expected    cosiapi.BootloaderType
		expectedPkg string
		errorText   string
	}{
		{"grub", []string{"grub2-efi-binary"}, cosiapi.BootloaderTypeGrub, "grub2-efi-binary", ""},
		{"systemd", []string{"systemd-boot"}, cosiapi.BootloaderTypeSystemdBoot, "systemd-boot", ""},
		{"ordinary-grub-precedence", []string{"grub2-efi-binary", "systemd-boot"},
			cosiapi.BootloaderTypeGrub, "grub2-efi-binary", ""},
		{"efitreeloader-mshv", []string{"efitreeloader", "edk2-hvloader", "mshv-bootloader-lx", "systemd-boot"},
			cosiapi.BootloaderTypeEfitreeloaderMshvSystemdBoot, "systemd-boot", ""},
		{"efitreeloader-mshv-before-grub", []string{"grub2-efi-binary", "efitreeloader", "edk2-hvloader", "mshv-bootloader-lx", "systemd-boot"},
			cosiapi.BootloaderTypeEfitreeloaderMshvSystemdBoot, "systemd-boot", ""},
		{"efitreeloader-mshv-unsigned-systemd", []string{"efitreeloader", "edk2-hvloader", "mshv-bootloader-lx", "systemd-boot-unsigned"},
			cosiapi.BootloaderTypeEfitreeloaderMshvSystemdBoot, "systemd-boot-unsigned", ""},
		{"missing-systemd", []string{"efitreeloader", "edk2-hvloader", "mshv-bootloader-lx", "grub2-efi-binary"},
			"", "", "requires systemd-boot"},
		{"unknown", nil, "", "", "unknown bootloader"},
	} {
		t.Run(test.name, func(t *testing.T) {
			kind, pkg, err := detectBootloaderTypeWithPackageQuery(func(name string) (bool, error) {
				return slices.Contains(test.packages, name), nil
			}, []string{"grub2-efi-binary"}, []string{"systemd-boot", "systemd-boot-unsigned"})
			if test.errorText != "" {
				require.ErrorContains(t, err, test.errorText)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.expected, kind)
			assert.Equal(t, test.expectedPkg, pkg)
		})
	}
}

func TestDetectBootloaderTypePackageQueryError(t *testing.T) {
	expected := errors.New("package query failed")
	for _, test := range []struct {
		name     string
		packages []string
		failed   string
	}{
		{"efitreeloader", nil, "efitreeloader"},
		{"hvloader", []string{"efitreeloader"}, "edk2-hvloader"},
		{"lxloader", []string{"efitreeloader", "edk2-hvloader"}, "mshv-bootloader-lx"},
		{"efitreeloader-mshv-systemd", []string{"efitreeloader", "edk2-hvloader", "mshv-bootloader-lx"}, "systemd-boot"},
		{"ordinary-grub", nil, "grub2-efi-binary"},
		{"ordinary-systemd", nil, "systemd-boot"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := detectBootloaderTypeWithPackageQuery(func(name string) (bool, error) {
				if name == test.failed {
					return false, expected
				}
				return slices.Contains(test.packages, name), nil
			}, []string{"grub2-efi-binary"}, []string{"systemd-boot", "systemd-boot-unsigned"})
			require.ErrorIs(t, err, expected)
		})
	}
}

func TestExtractCosiBootMetadataStandaloneUki(t *testing.T) {
	stubPath, addonStubPath := checkSkipForUkiStubs(t)
	for _, test := range []struct {
		name           string
		bootloaderType cosiapi.BootloaderType
	}{
		{"systemd", cosiapi.BootloaderTypeSystemdBoot},
		{"efitreeloader-mshv", cosiapi.BootloaderTypeEfitreeloaderMshvSystemdBoot},
	} {
		t.Run(test.name, func(t *testing.T) {
			testExtractCosiBootMetadataStandaloneUki(t, stubPath, addonStubPath, test.bootloaderType)
		})
	}
}

func testExtractCosiBootMetadataStandaloneUki(t *testing.T, stubPath, addonStubPath string,
	bootloaderType cosiapi.BootloaderType,
) {
	root := t.TempDir()
	buildDir := t.TempDir()
	espDir := filepath.Join(root, "boot/efi")
	kernel := "vmlinuz-6.6.1"
	osRelease := filepath.Join(buildDir, "os-release")
	require.NoError(t, os.MkdirAll(filepath.Join(buildDir, UkiBuildDir), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(espDir, UkiOutputDir), 0o755))
	require.NoError(t, file.Copy(stubPath, filepath.Join(buildDir, UkiBuildDir, kernel)))
	require.NoError(t, os.WriteFile(filepath.Join(buildDir, UkiBuildDir, "initramfs.img"), []byte("initramfs"), 0o644))
	require.NoError(t, os.WriteFile(osRelease, []byte("ID=azurelinux\n"), 0o644))
	require.NoError(t, buildMainUki(kernel, "initramfs.img", "root=LABEL=ROOT ro", osRelease,
		stubPath, buildDir, espDir, "6.6.1"))
	require.NoError(t, buildUkiAddon(kernel, "test.addon.efi", "console=ttyS0", addonStubPath, espDir))

	connection := imageconnection.NewImageConnection()
	require.NoError(t, connection.ConnectChroot(root, true, nil, nil, false))
	defer connection.Close()
	metadata, err := extractCosiBootMetadata(buildDir, connection,
		&cosiBootMetadataTestDistro{bootloaderType: bootloaderType}, nil)
	require.NoError(t, err)
	assert.Equal(t, &cosiapi.Bootloader{
		Type: bootloaderType,
		SystemdBoot: &cosiapi.SystemDBoot{Entries: []cosiapi.SystemDBootEntry{{
			Type: cosiapi.SystemDBootEntryTypeUKIStandalone, Path: "/boot/efi/EFI/Linux/vmlinuz-6.6.1.efi",
			Kernel: "6.6.1", Cmdline: "root=LABEL=ROOT ro console=ttyS0",
		}}},
	}, metadata)
}

func TestExtractCosiBootMetadataRequiresSystemdEntries(t *testing.T) {
	connection := imageconnection.NewImageConnection()
	require.NoError(t, connection.ConnectChroot(t.TempDir(), true, nil, nil, false))
	defer connection.Close()
	distro := &cosiBootMetadataTestDistro{
		bootloaderType: cosiapi.BootloaderTypeEfitreeloaderMshvSystemdBoot,
	}
	_, err := extractCosiBootMetadata(t.TempDir(), connection, distro, nil)
	require.ErrorContains(t, err, "no supported boot entries")
}

func TestExtractCosiBootMetadataPreservesSystemdEntries(t *testing.T) {
	for _, test := range []struct {
		name           string
		bootloaderType cosiapi.BootloaderType
	}{
		{"systemd", cosiapi.BootloaderTypeSystemdBoot},
		{"efitreeloader-mshv", cosiapi.BootloaderTypeEfitreeloaderMshvSystemdBoot},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			entryDir := filepath.Join(root, "boot/loader/entries")
			require.NoError(t, os.MkdirAll(entryDir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(entryDir, "test.conf"),
				[]byte("linux /vmlinuz-6.6.1\noptions root=LABEL=ROOT ro\n"), 0o644))
			connection := imageconnection.NewImageConnection()
			require.NoError(t, connection.ConnectChroot(root, true, nil, nil, false))
			defer connection.Close()
			distro := &cosiBootMetadataTestDistro{bootloaderType: test.bootloaderType}
			metadata, err := extractCosiBootMetadata(t.TempDir(), connection, distro, nil)
			require.NoError(t, err)
			assert.Equal(t, &cosiapi.Bootloader{
				Type: test.bootloaderType,
				SystemdBoot: &cosiapi.SystemDBoot{Entries: []cosiapi.SystemDBootEntry{{
					Type: cosiapi.SystemDBootEntryTypeConfig, Path: "/boot/loader/entries/test.conf",
					Kernel: "6.6.1", Cmdline: "root=LABEL=ROOT ro",
				}}},
			}, metadata)
		})
	}
}

func TestBuildCosiBootloaderMetadataVersion(t *testing.T) {
	for _, test := range []struct {
		name       string
		bootloader cosiapi.Bootloader
		version    string
	}{
		{"grub", cosiapi.Bootloader{Type: cosiapi.BootloaderTypeGrub}, "1.2"},
		{"systemd", cosiapi.Bootloader{Type: cosiapi.BootloaderTypeSystemdBoot,
			SystemdBoot: &cosiapi.SystemDBoot{Entries: []cosiapi.SystemDBootEntry{{
				Type: cosiapi.SystemDBootEntryTypeUKIStandalone, Path: "/boot/efi/EFI/Linux/test.efi",
				Kernel: "6.6.1", Cmdline: "root=LABEL=ROOT ro",
			}}}}, "1.2"},
		{"efitreeloader-mshv", cosiapi.Bootloader{Type: cosiapi.BootloaderTypeEfitreeloaderMshvSystemdBoot,
			SystemdBoot: &cosiapi.SystemDBoot{Entries: []cosiapi.SystemDBootEntry{{
				Type: cosiapi.SystemDBootEntryTypeUKIStandalone, Path: "/boot/efi/EFI/Linux/test.efi",
				Kernel: "6.6.1", Cmdline: "root=LABEL=ROOT ro",
			}}}}, "1.3"},
	} {
		t.Run(test.name, func(t *testing.T) {
			testBuildCosiBootloaderMetadata(t, test.bootloader, test.version)
		})
	}
}

func testBuildCosiBootloaderMetadata(t *testing.T, bootloader cosiapi.Bootloader, version string) {
	dir := t.TempDir()
	encoder, err := zstd.NewWriter(nil)
	require.NoError(t, err)
	defer encoder.Close()
	data := encoder.EncodeAll(make([]byte, 4096), nil)
	for _, name := range []string{"gpt.raw.zst", "esp.raw.zst"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, 0o644))
	}
	partitions := []outputPartitionMetadata{{
		PartitionNum: 1, PartitionFilename: "esp.raw.zst", PartUuid: "esp",
		FileSystemType: "vfat", Uuid: "A1B2-C3D4", PartitionTypeUuid: diskutils.EfiSystemPartitionTypeUuid,
		UncompressedSize: 4096,
	}}
	gpt := &GptExtractedData{
		CompressedFilePath: filepath.Join(dir, "gpt.raw.zst"), UncompressedSize: 4096,
		DiskSize: 1024 * 1024,
		PartitionTable: &diskutils.PartitionTable{SectorSize: 512,
			Partitions: []diskutils.PartitionTablePartition{{Path: "/dev/loop0p1"}}},
	}
	output := filepath.Join(dir, "test.cosi")
	require.NoError(t, buildCosiFile(dir, output, partitions, nil, []fstabEntryPartNum{{
		PartUuid: "esp", FstabEntry: diskutils.FstabEntry{Target: "/boot/efi"},
	}}, "11111111-2222-3333-4444-555555555555", "ID=azurelinux\n", []cosiapi.OsPackage{}, &bootloader, gpt, 0))
	_, metadata, err := extractCosi(output, filepath.Join(dir, "extracted"))
	require.NoError(t, err)
	assert.Equal(t, version, metadata.Version)
	assert.Equal(t, bootloader, metadata.Bootloader)
}
