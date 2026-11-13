// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
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

type bootloaderTestDistro struct {
	DistroHandler
	packages    []string
	failedQuery string
	queryError  error
}

func (d *bootloaderTestDistro) IsPackageInstalled(_ safechroot.ChrootInterface, _ *safechroot.Chroot,
	name string,
) (bool, error) {
	if name == d.failedQuery {
		return false, d.queryError
	}
	for _, pkg := range d.packages {
		if pkg == name {
			return true, nil
		}
	}
	return false, nil
}

func (d *bootloaderTestDistro) DetectBootloaderType(chroot safechroot.ChrootInterface,
	tools *safechroot.Chroot,
) (cosiapi.BootloaderType, error) {
	kind, _, err := detectBootloaderType(d, chroot, tools,
		[]string{"grub2-efi-binary"}, []string{"systemd-boot", "systemd-boot-unsigned"})
	return kind, err
}

func (*bootloaderTestDistro) GetEspDir() string {
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
		{"mshv", []string{"efitreeloader", "edk2-hvloader", "mshv-bootloader-lx", "systemd-boot"},
			cosiapi.BootloaderTypeEfitreeloaderMshvSystemdBoot, "systemd-boot", ""},
		{"mshv-before-grub", []string{"grub2-efi-binary", "efitreeloader", "edk2-hvloader", "mshv-bootloader-lx", "systemd-boot"},
			cosiapi.BootloaderTypeEfitreeloaderMshvSystemdBoot, "systemd-boot", ""},
		{"mshv-unsigned-systemd", []string{"efitreeloader", "edk2-hvloader", "mshv-bootloader-lx", "systemd-boot-unsigned"},
			cosiapi.BootloaderTypeEfitreeloaderMshvSystemdBoot, "systemd-boot-unsigned", ""},
		{"runtime-is-not-boot-chain", []string{"mshv", "kernel-mshv", "systemd-boot"},
			cosiapi.BootloaderTypeSystemdBoot, "systemd-boot", ""},
		{"efitreeloader-only", []string{"efitreeloader", "systemd-boot"},
			cosiapi.BootloaderTypeSystemdBoot, "systemd-boot", ""},
		{"missing-hvloader", []string{"efitreeloader", "mshv-bootloader-lx", "systemd-boot"},
			cosiapi.BootloaderTypeSystemdBoot, "systemd-boot", ""},
		{"missing-lxloader", []string{"efitreeloader", "edk2-hvloader", "systemd-boot"},
			cosiapi.BootloaderTypeSystemdBoot, "systemd-boot", ""},
		{"missing-efitreeloader", []string{"edk2-hvloader", "mshv-bootloader-lx", "systemd-boot"},
			cosiapi.BootloaderTypeSystemdBoot, "systemd-boot", ""},
		{"missing-systemd", []string{"efitreeloader", "edk2-hvloader", "mshv-bootloader-lx", "grub2-efi-binary"},
			"", "", "requires systemd-boot"},
		{"unknown", nil, "", "", "unknown bootloader"},
	} {
		t.Run(test.name, func(t *testing.T) {
			distro := &bootloaderTestDistro{packages: test.packages}
			kind, pkg, err := detectBootloaderType(distro, nil, nil,
				[]string{"grub2-efi-binary"}, []string{"systemd-boot", "systemd-boot-unsigned"})
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
		{"mshv-systemd", []string{"efitreeloader", "edk2-hvloader", "mshv-bootloader-lx"}, "systemd-boot"},
		{"ordinary-grub", nil, "grub2-efi-binary"},
		{"ordinary-systemd", nil, "systemd-boot"},
	} {
		t.Run(test.name, func(t *testing.T) {
			distro := &bootloaderTestDistro{
				packages:    test.packages,
				failedQuery: test.failed, queryError: expected,
			}
			_, err := distro.DetectBootloaderType(nil, nil)
			require.ErrorIs(t, err, expected)
		})
	}
}

func TestExtractCosiBootMetadataStandaloneUki(t *testing.T) {
	stubPath, addonStubPath := checkSkipForUkiStubs(t)
	for _, test := range []struct {
		name     string
		packages []string
		expected cosiapi.BootloaderType
	}{
		{"systemd", []string{"systemd-boot"}, cosiapi.BootloaderTypeSystemdBoot},
		{"mshv", []string{"efitreeloader", "edk2-hvloader", "mshv-bootloader-lx", "systemd-boot"},
			cosiapi.BootloaderTypeEfitreeloaderMshvSystemdBoot},
	} {
		t.Run(test.name, func(t *testing.T) {
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
				&bootloaderTestDistro{packages: test.packages}, nil)
			require.NoError(t, err)
			assert.Equal(t, &cosiapi.Bootloader{
				Type: test.expected,
				SystemdBoot: &cosiapi.SystemDBoot{Entries: []cosiapi.SystemDBootEntry{{
					Type: cosiapi.SystemDBootEntryTypeUKIStandalone, Path: "/boot/efi/EFI/Linux/vmlinuz-6.6.1.efi",
					Kernel: "6.6.1", Cmdline: "root=LABEL=ROOT ro console=ttyS0",
				}}},
			}, metadata)
		})
	}
}

func TestAclBootloaderWithoutPackageMetadata(t *testing.T) {
	distro := &aclDistroHandler{}
	kind, err := distro.DetectBootloaderType(safechroot.NewChroot(t.TempDir(), true), nil)
	require.NoError(t, err)
	assert.Equal(t, cosiapi.BootloaderTypeSystemdBoot, kind)
}

func TestExtractCosiBootMetadataRequiresSystemdEntries(t *testing.T) {
	connection := imageconnection.NewImageConnection()
	require.NoError(t, connection.ConnectChroot(t.TempDir(), true, nil, nil, false))
	defer connection.Close()
	distro := &bootloaderTestDistro{
		packages: []string{"efitreeloader", "edk2-hvloader", "mshv-bootloader-lx", "systemd-boot"},
	}
	_, err := extractCosiBootMetadata(t.TempDir(), connection, distro, nil)
	require.ErrorContains(t, err, "no supported boot entries")
}

func TestExtractCosiBootMetadataPreservesSystemdEntries(t *testing.T) {
	for _, test := range []struct {
		name     string
		packages []string
		expected cosiapi.BootloaderType
	}{
		{"systemd", []string{"systemd-boot"}, cosiapi.BootloaderTypeSystemdBoot},
		{"mshv", []string{"efitreeloader", "edk2-hvloader", "mshv-bootloader-lx", "systemd-boot"},
			cosiapi.BootloaderTypeEfitreeloaderMshvSystemdBoot},
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
			distro := &bootloaderTestDistro{packages: test.packages}
			metadata, err := extractCosiBootMetadata(t.TempDir(), connection, distro, nil)
			require.NoError(t, err)
			assert.Equal(t, &cosiapi.Bootloader{
				Type: test.expected,
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
		{"mshv", cosiapi.Bootloader{Type: cosiapi.BootloaderTypeEfitreeloaderMshvSystemdBoot,
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
	file, err := os.Open(output)
	require.NoError(t, err)
	defer file.Close()
	reader := tar.NewReader(file)
	for {
		header, err := reader.Next()
		require.NoError(t, err, "metadata.json must be present in COSI")
		if header.Name != CosiMetadataName {
			continue
		}
		body, err := io.ReadAll(reader)
		require.NoError(t, err)
		var metadata cosiapi.MetadataJson
		require.NoError(t, json.Unmarshal(body, &metadata))
		assert.Equal(t, version, metadata.Version)
		assert.Equal(t, bootloader, metadata.Bootloader)
		break
	}
}
