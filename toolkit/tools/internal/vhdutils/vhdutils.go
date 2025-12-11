// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package vhdutils

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"time"

	"github.com/google/uuid"
)

const (
	VhdFileFooterSize = 512
	VhdFileSignature  = "conectix"
	VhdFileVersion    = 0x00010000
)

type VhdFileFooter struct {
	Cookie             [8]byte
	Features           uint32
	FileFormatVersion  uint32
	DataOffset         uint64
	TimeStamp          uint32
	CreatorApplication [4]byte
	CreatorVersion     [4]byte
	CreatorHostOS      [4]byte
	OriginalSize       uint64
	CurrentSize        uint64
	Cylinder           uint16
	Heads              uint8
	SectorsPerCylinder uint8
	DiskType           uint32
	Checksum           [4]byte
	UniqueId           [16]byte
	SavedState         uint8
	Reserved           [427]byte
}

var (
	ErrVhdFileTooSmall       = errors.New("file is too small to be a VHD")
	ErrVhdWrongFileSignature = errors.New("footer does not have correct VHD file signature")
	ErrVhdWrongFileVersion   = errors.New("VHD footer has unsupported file format version")
)

type VhdFileSizeCalcType int

const (
	// File is not a VHD file.
	VhdFileSizeCalcTypeNone VhdFileSizeCalcType = iota
	// VHD uses "Current Size" field to calculate the disk size.
	VhdFileSizeCalcTypeCurrentSize
	// VHD uses "Disk Geometry" fields to calculate the disk size.
	VhdFileSizeCalcTypeDiskGeometry
)

func GetVhdFileSizeCalcType(filename string) (VhdFileSizeCalcType, error) {
	footer, err := ParseVhdFileFooter(filename)
	if errors.Is(err, ErrVhdFileTooSmall) || errors.Is(err, ErrVhdWrongFileSignature) {
		// Not a VHD file.
		return VhdFileSizeCalcTypeNone, nil
	}
	if err != nil {
		return VhdFileSizeCalcTypeNone, err
	}

	creatorApplication := string(footer.CreatorApplication[:])

	//   There are actually two different ways of calculating the disk size of a VHD file. The old method, which is
	// used by Microsoft Virtual PC, uses the VHD's footer's 'Disk Geometry' (cylinder, heads, and sectors per
	// track/cylinder) fields. Using 'Disk Geometry' limits what file sizes are possible. So, Hyper-V uses only uses the
	// the 'Current Size' field, which allows it to accept disks of any size.
	//   Microsoft Virtual PC is pretty dead at this point. So, it is fairly safe to assume that almost all VHD files
	// use the Hyper-V format. Unfortunately, qemu-img still defaults to using 'Disk Geometry' when a user requests a
	// VHD (i.e. 'vpc') image. Image Customizer knows to pass the 'force_size=on' arg to qemu-img so that it uses
	// 'Current Size'. But users might not know that they need to do this when using qemu-img manually.
	//   Fortunately, qemu-img is nice enough to use different values of the 'Creator Application' field depending on
	// the value of 'force_size'. Specifically, "qemu" for 'Disk Geometry' and "qem2 " for 'Current Size'. This can be
	// used to determine which type of VHD we are dealing with.
	//   qemu-img uses the 'Creator Application' field internally to determine what type of VHD it is dealing with.
	// However, if it sees a 'Creator Application' value it doesn't recognize, it will assume it uses 'Disk Geometry'.
	// Whereas, nowadays it is more likely for a VHD to use 'Current Size'.
	switch creatorApplication {
	case "vpc ", "vs  ", "qemu":
		return VhdFileSizeCalcTypeDiskGeometry, nil

	default:
		return VhdFileSizeCalcTypeCurrentSize, nil
	}
}

func ParseVhdFileFooter(filename string) (VhdFileFooter, error) {
	fd, err := os.Open(filename)
	if err != nil {
		return VhdFileFooter{}, err
	}
	defer fd.Close()

	stat, err := fd.Stat()
	if err != nil {
		return VhdFileFooter{}, err
	}

	if stat.Size() < VhdFileFooterSize {
		return VhdFileFooter{}, ErrVhdFileTooSmall
	}

	_, err = fd.Seek(-VhdFileFooterSize, io.SeekEnd)
	if err != nil {
		return VhdFileFooter{}, err
	}

	footerBytes := [VhdFileFooterSize]byte{}
	_, err = fd.Read([]byte(footerBytes[:]))
	if err != nil {
		return VhdFileFooter{}, err
	}

	footerReader := bytes.NewReader(footerBytes[:])

	var footer VhdFileFooter
	err = binary.Read(footerReader, binary.BigEndian, &footer)
	if err != nil {
		return VhdFileFooter{}, err
	}

	if string(footer.Cookie[:]) != VhdFileSignature {
		return VhdFileFooter{}, ErrVhdWrongFileSignature
	}

	if footer.FileFormatVersion != VhdFileVersion {
		return VhdFileFooter{}, ErrVhdWrongFileVersion
	}

	return footer, nil
}

func AppendVhdFileFooter(fd io.Writer, fileSize uint64) error {
	const megaByte = 1024 * 1024

	padding := uint64(0)
	if fileSize%megaByte != 0 {
		padding = megaByte - (fileSize % megaByte)
	}

	_, err := fd.Write(make([]byte, padding))
	if err != nil {
		return err
	}

	footer, err := createVpcFooter(fileSize + padding)
	if err != nil {
		return err
	}

	_, err = fd.Write(footer[:])
	if err != nil {
		return err
	}

	return nil
}

// createVpcFooter creates a fixed-sized VHD footer according to the VHD specification.
// The footer is 512 bytes and contains metadata about the virtual hard disk.
func createVpcFooter(fileSize uint64) ([VhdFileFooterSize]byte, error) {
	footer := [VhdFileFooterSize]byte{}
	offset := 0

	// Cookie (8 bytes): "conectix" - identifies this as a Microsoft VHD
	copy(footer[offset:], []byte("conectix"))
	offset += 8

	// Features (4 bytes): 0x00000002 (Reserved bit must be set)
	binary.BigEndian.PutUint32(footer[offset:], 0x00000002)
	offset += 4

	// File Format Version (4 bytes): 0x00010000 (major.minor = 1.0)
	binary.BigEndian.PutUint32(footer[offset:], 0x00010000)
	offset += 4

	// Data Offset (8 bytes): 0xFFFFFFFFFFFFFFFF for fixed disks
	binary.BigEndian.PutUint64(footer[offset:], 0xFFFFFFFFFFFFFFFF)
	offset += 8

	// Time Stamp (4 bytes): seconds since January 1, 2000 00:00:00 UTC
	y2k := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	timestamp := uint32(time.Now().UTC().Sub(y2k).Seconds())
	binary.BigEndian.PutUint32(footer[offset:], timestamp)
	offset += 4

	// Creator Application (4 bytes): "vpc " for Virtual PC
	copy(footer[offset:], []byte("vpc "))
	offset += 4

	// Creator Version (4 bytes): 0x00050000 (Virtual PC 2004)
	binary.BigEndian.PutUint32(footer[offset:], 0x00050000)
	offset += 4

	// Creator Host OS (4 bytes): "Wi2k" for Windows
	copy(footer[offset:], []byte("Wi2k"))
	offset += 4

	// Original Size (8 bytes): size of the hard disk in bytes
	binary.BigEndian.PutUint64(footer[offset:], fileSize)
	offset += 8

	// Current Size (8 bytes): same as original size for fixed disks
	binary.BigEndian.PutUint64(footer[offset:], fileSize)
	offset += 8

	// Disk Geometry (4 bytes): Calculate CHS values
	geometry := calculateDiskGeometry(fileSize)
	binary.BigEndian.PutUint16(footer[offset:], geometry.Cylinders)
	footer[offset+2] = geometry.Heads
	footer[offset+3] = geometry.SectorsPerTrack
	offset += 4

	// Disk Type (4 bytes): 2 for fixed hard disk
	binary.BigEndian.PutUint32(footer[offset:], 2)
	offset += 4

	// Checksum (4 bytes): will be calculated after setting other fields
	checksumOffset := offset
	offset += 4

	// Unique ID (16 bytes): UUID for this disk
	diskUUID, err := uuid.NewRandom()
	if err != nil {
		return footer, err
	}
	copy(footer[offset:], diskUUID[:])
	offset += 16

	// Saved State (1 byte): 0 (not in saved state)
	footer[offset] = 0
	offset += 1

	// Reserved (427 bytes): all zeroes (already initialized)
	// offset += 427

	// Calculate and set checksum (one's complement of sum of all bytes except checksum)
	checksum := uint32(0)
	for i := 0; i < VhdFileFooterSize; i++ {
		if i < checksumOffset || i >= checksumOffset+4 {
			checksum += uint32(footer[i])
		}
	}
	checksum = ^checksum
	binary.BigEndian.PutUint32(footer[checksumOffset:], checksum)

	return footer, nil
}

// DiskGeometry represents CHS (Cylinder/Head/Sector) values
type DiskGeometry struct {
	Cylinders       uint16
	Heads           uint8
	SectorsPerTrack uint8
}

// calculateDiskGeometry calculates CHS values from disk size. This follows the
// algorithm from the VHD specification appendix in
// https://www.microsoft.com/en-us/download/details.aspx?id=23850
func calculateDiskGeometry(totalSize uint64) DiskGeometry {
	const sectorSize = 512

	// Get total sectors
	totalSectors := totalSize / sectorSize

	// If there is a remainder, add one more sector
	if totalSize%sectorSize != 0 {
		totalSectors++
	}

	var heads, sectorsPerTrack, cylinderTimesHeads uint64

	// If total sectors > 65535 * 16 * 255
	if totalSectors > 267382800 {
		totalSectors = 267382800
	}

	// Calculate sectors per track
	if totalSectors >= 66059280 { // > 65535 * 16 * 63
		sectorsPerTrack = 255
		heads = 16
		cylinderTimesHeads = totalSectors / sectorsPerTrack
	} else {
		sectorsPerTrack = 17
		cylinderTimesHeads = totalSectors / sectorsPerTrack
		heads = (cylinderTimesHeads + 1023) / 1024

		if heads < 4 {
			heads = 4
		}
		if cylinderTimesHeads >= (heads*1024) || heads > 16 {
			sectorsPerTrack = 31
			heads = 16
			cylinderTimesHeads = totalSectors / sectorsPerTrack
		}
		if cylinderTimesHeads >= (heads * 1024) {
			sectorsPerTrack = 63
			heads = 16
			cylinderTimesHeads = totalSectors / sectorsPerTrack
		}
	}

	cylinders := cylinderTimesHeads / heads

	// Ensure values fit in their respective field sizes
	if cylinders > 65535 {
		cylinders = 65535
	}

	if sectorsPerTrack > 255 {
		sectorsPerTrack = 255
	}

	if heads > 16 {
		heads = 16
	}

	return DiskGeometry{
		Cylinders:       uint16(cylinders),
		Heads:           uint8(heads),
		SectorsPerTrack: uint8(sectorsPerTrack),
	}
}
