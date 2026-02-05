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
	Version     string         `json:"version"`
	OsArch      string         `json:"osArch"`
	Disk        *Disk          `json:"disk,omitempty"`
	Images      []FileSystem   `json:"images"`
	OsRelease   string         `json:"osRelease"`
	Id          string         `json:"id,omitempty"`
	Bootloader  CosiBootloader `json:"bootloader"`
	OsPackages  []OsPackage    `json:"osPackages"`
	Compression *Compression   `json:"compression,omitempty"`
}

type Compression struct {
	MaxWindowLog int `json:"maxWindowLog"`
}

type DiskType string

const (
	DiskTypeGpt DiskType = "gpt"
)

type RegionType string

const (
	RegionTypePrimaryGpt  RegionType = "primary-gpt"
	RegionTypePartition   RegionType = "partition"
	RegionTypeBackupGpt   RegionType = "backup-gpt"
	RegionTypeUnallocated RegionType = "unallocated"
	RegionTypeUnknown     RegionType = "unknown"
)

type Disk struct {
	Size       uint64          `json:"size"`       // Size of the original disk in bytes
	Type       DiskType        `json:"type"`       // Partitioning type "gpt"
	LbaSize    int             `json:"lbaSize"`    // Logical block address size in bytes
	GptRegions []GptDiskRegion `json:"gptRegions"` // Regions in the GPT disk
}

type GptDiskRegion struct {
	Image    ImageFile  `json:"image"`              // Details of the image file in the tarball
	Type     RegionType `json:"type"`               // The type of region this image represents
	StartLba *int64     `json:"startLba,omitempty"` // The first LBA of the region
	Number   *int       `json:"number,omitempty"`   // Partition number
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

type OsPackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Release string `json:"release"`
	Arch    string `json:"arch"`
}
