package imagecustomizerlib

type MetadataJson struct {
	Version   string  `json:"version"`
	OsArch    string  `json:"osArch"`
	Images    []Image `json:"images"`
	OsRelease string  `json:"osRelease"`
	Id        string  `json:"id"`
}

type Image struct {
	Image      ImageFile     `json:"image"`
	MountPoint string        `json:"mountPoint"`
	FsType     string        `json:"fsType"`
	FsUuid     string        `json:"fsUuid"`
	PartType   PartitionType `json:"partType"`
	Verity     *Verity       `json:"verity"`
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

type PartitionType string

const (
	PartitionTypeEsp                PartitionType = "c12a7328-f81f-11d2-ba4b-00a0c93ec93b"
	PartitionTypeXbootldr           PartitionType = "bc13c2ff-59e6-4262-a352-b275fd6f7172"
	PartitionTypeSwap               PartitionType = "0657fd6d-a4ab-43c4-84e5-0933c84b4f4f"
	PartitionTypeHome               PartitionType = "933ac7e1-2eb4-4f13-b844-0e14e2aef915"
	PartitionTypeSrv                PartitionType = "3b8f8425-20e0-4f3b-907f-1a25a76f98e8"
	PartitionTypeVar                PartitionType = "4d21b016-b534-45c2-a9fb-5c16e091fd2d"
	PartitionTypeTmp                PartitionType = "7ec6f557-3bc5-4aca-b293-16ef5df639d1"
	PartitionTypeLinuxGeneric       PartitionType = "0fc63daf-8483-4772-8e79-3d69d8477de4"
	PartitionTypeRootAmd64          PartitionType = "4f68bce3-e8cd-4db1-96e7-fbcaf984b709"
	PartitionTypeRootAmd64Verity    PartitionType = "2c7357ed-ebd2-46d9-aec1-23d437ec2bf5"
	PartitionTypeRootAmd64VeritySig PartitionType = "41092b05-9fc8-4523-994f-2def0408b176"
	PartitionTypeUsrAmd64           PartitionType = "8484680c-9521-48c6-9c11-b0720656f69e"
	PartitionTypeUsrAmd64Verity     PartitionType = "77ff5f63-e7b6-4633-acf4-1565b864c0e6"
	PartitionTypeUsrAmd64VeritySig  PartitionType = "e7bb33fb-06cf-4e81-8273-e543b413e2e2"
	PartitionTypeRootArm64          PartitionType = "b921b045-1df0-41c3-af44-4c6f280d3fae"
	PartitionTypeRootArm64Verity    PartitionType = "df3300ce-d69f-4c92-978c-9bfb0f38d820"
	PartitionTypeRootArm64VeritySig PartitionType = "6db69de6-29f4-4758-a7a5-962190f00ce3"
	PartitionTypeUsrArm64           PartitionType = "b0e01050-ee5f-4390-949a-9101b17104e9"
	PartitionTypeUsrArm64Verity     PartitionType = "6e11a4e7-fbca-4ded-b9e9-e1a512bb664e"
	PartitionTypeUsrArm64VeritySig  PartitionType = "c23ce4ff-44bd-4b00-b2d4-b41b3419e02a"

	PartitionTypeRoot          PartitionType = PartitionTypeRootAmd64
	PartitionTypeRootVerity    PartitionType = PartitionTypeRootAmd64Verity
	PartitionTypeRootVeritySig PartitionType = PartitionTypeRootAmd64VeritySig
	PartitionTypeUsr           PartitionType = PartitionTypeUsrAmd64
	PartitionTypeUsrVerity     PartitionType = PartitionTypeUsrAmd64Verity
	PartitionTypeUsrVeritySig  PartitionType = PartitionTypeUsrAmd64VeritySig
)
