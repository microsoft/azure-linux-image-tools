// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

//

package configuration

// DestinationFileConfigList is a list of destination files where the source file will be copied to in the final image.
// This type exists to allow a custom marshaller to be attached to it.
type FileConfigList []FileConfig

// FileConfig specifies options for how a file is copied in the target OS.
type FileConfig struct {
	// The file path in the target OS that the file will be copied to.
	Path string `json:"Path"`

	// The file permissions to set on the file.
	Permissions *FilePermissions `json:"Permissions"`
}
