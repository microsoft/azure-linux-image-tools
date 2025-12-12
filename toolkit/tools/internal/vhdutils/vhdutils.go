// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package vhdutils

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
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
