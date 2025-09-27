// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Parser for the image builder's configuration schemas.

package configuration

// Artifact [non-ISO image building only] defines the name, type
// and optional compression of the output Azure Linux image.
type Artifact struct {
	Compression string `json:"Compression"`
	Name        string `json:"Name"`
	Type        string `json:"Type"`
}

// RawBinary allow the users to specify a binary they would
// like to copy byte-for-byte onto the disk.
type RawBinary struct {
	BinPath   string `json:"BinPath"`
	BlockSize uint64 `json:"BlockSize"`
	Seek      uint64 `json:"Seek"`
}

// TargetDisk [kickstart-only] defines the physical disk, to which
// Azure Linux should be installed.
type TargetDisk struct {
	Type  string `json:"Type"`
	Value string `json:"Value"`
}

// InstallScript defines a script to be run before or after other installation
// steps and provides a way to pass parameters to it.
type InstallScript struct {
	Args string `json:"Args"`
	Path string `json:"Path"`
}

// Group defines a single group to be created on the new system.
type Group struct {
	Name string `json:"Name"`
	GID  string `json:"GID"`
}

// RootEncryption enables encryption on the root partition
type RootEncryption struct {
	Enable   bool   `json:"Enable"`
	Password string `json:"Password"`
}
