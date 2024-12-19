package imagecustomizerlib

type MetadataJson struct {
	Version   string  `json:"version"`
	OsArch    string  `json:"osArch"`
	Images    []Image `json:"images"`
	OsRelease string  `json:"osRelease"`
	Id        string  `json:"id"`
}

type Image struct {
	Image            ImageFile `json:"image"`
	MountPoint       string    `json:"mountPoint"`
	FsType           string    `json:"fsType"`
	FsUuid           string    `json:"fsUuid"`
	PartType         string    `json:"partType"`
	Verity           *Verity   `json:"verity"`
	UncompressedSize uint64    `json:"uncompressedSize"`
}

type Verity struct {
	Image    ImageFile `json:"image"`
	Roothash string    `json:"roothash"`
}

type ImageFile struct {
	Path             string `json:"path"`
	CompressedSize   uint64 `json:"compressedSize"`
	UncompressedSize uint64 `json:"uncompressedSize"`
	Sha384           string `json:"sha384"`
}
