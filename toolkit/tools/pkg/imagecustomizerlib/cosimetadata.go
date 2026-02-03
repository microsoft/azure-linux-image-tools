package imagecustomizerlib

type BootloaderType string

const (
	BootloaderTypeGrub        BootloaderType = "grub"
	BootloaderTypeSystemdBoot BootloaderType = "systemd-boot"
)

type SystemDBootEntryType string

const (
	SystemDBootEntryTypeUKIStandalone SystemDBootEntryType = "uki-standalone"
	SystemDBootEntryTypeUKIConfig     SystemDBootEntryType = "uki-config"
	SystemDBootEntryTypeConfig        SystemDBootEntryType = "config"
)

type SystemDBootEntry struct {
	Type    SystemDBootEntryType `json:"type"`
	Path    string               `json:"path"`
	Cmdline string               `json:"cmdline"`
	Kernel  string               `json:"kernel"`
}

type SystemDBoot struct {
	Entries []SystemDBootEntry `json:"entries"`
}

type CosiBootloader struct {
	Type        BootloaderType `json:"type"`
	SystemdBoot *SystemDBoot   `json:"systemdBoot,omitempty"`
}

type MetadataJson struct {
	Version    string         `json:"version"`
	OsArch     string         `json:"osArch"`
	Disk       *Disk          `json:"disk,omitempty"`
	Images     []FileSystem   `json:"images"`
	Partitions []Partition    `json:"partitions"`
	OsRelease  string         `json:"osRelease"`
	Id         string         `json:"id,omitempty"`
	Bootloader CosiBootloader `json:"bootloader"`
	OsPackages []OsPackage    `json:"osPackages"`
}

// DiskType represents the partitioning table type
type DiskType string

const (
	DiskTypeGpt DiskType = "gpt"
)

// RegionType represents the type of region in the original disk image
type RegionType string

const (
	RegionTypePrimaryGpt  RegionType = "primary-gpt"
	RegionTypePartition   RegionType = "partition"
	RegionTypeBackupGpt   RegionType = "backup-gpt"
	RegionTypeUnallocated RegionType = "unallocated"
	RegionTypeUnknown     RegionType = "unknown"
)

// Disk holds information about the original disk layout
type Disk struct {
	Size       uint64          `json:"size"`       // Size of the original disk in bytes
	Type       DiskType        `json:"type"`       // Partitioning type ("gpt")
	LbaSize    int             `json:"lbaSize"`    // Logical block address size in bytes (usually 512)
	GptRegions []GptDiskRegion `json:"gptRegions"` // Regions in the GPT disk
}

// GptDiskRegion holds information about a specific region of the original disk image
type GptDiskRegion struct {
	Image    ImageFile  `json:"image"`              // Details of the image file in the tarball
	Type     RegionType `json:"type"`               // The type of region this image represents
	StartLba *int64     `json:"startLba,omitempty"` // The first LBA of the region (for backup-gpt, unallocated, unknown)
	Number   *int       `json:"number,omitempty"`   // Partition number (1-based, for partition type only)
}

type FileSystem struct {
	Image      ImageFile     `json:"image"`
	MountPoint string        `json:"mountPoint"`
	FsType     string        `json:"fsType"`
	FsUuid     string        `json:"fsUuid"`
	PartType   string        `json:"partType"`
	Verity     *VerityConfig `json:"verity"`
}

type VerityConfig struct {
	Image    ImageFile `json:"image"`
	Roothash string    `json:"roothash"`
}

type ImageFile struct {
	Path             string `json:"path"`
	CompressedSize   uint64 `json:"compressedSize"`
	UncompressedSize uint64 `json:"uncompressedSize"`
	Sha384           string `json:"sha384"`
}

type Partition struct {
	Image        ImageFile `json:"image"`
	OriginalSize uint64    `json:"originalSize"`
	PartUuid     string    `json:"partUuid"`
	Label        string    `json:"label"`
	Number       int       `json:"number"`
}

type OsPackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Release string `json:"release"`
	Arch    string `json:"arch"`
}
