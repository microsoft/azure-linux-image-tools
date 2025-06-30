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
	Version    string          `json:"version"`
	OsArch     string          `json:"osArch"`
	Images     []FileSystem    `json:"images"`
	OsRelease  string          `json:"osRelease"`
	Id         string          `json:"id,omitempty"`
	Bootloader *CosiBootloader `json:"bootloader"`
	OsPackages []OsPackage     `json:"osPackages"`
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
