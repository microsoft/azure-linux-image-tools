// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

//

package configuration

import (
	"os"
)

// FilePermissions represents the file permissions to set on a file.
//
// Accepted formats:
//
// - Octal string (e.g. "660")
type FilePermissions os.FileMode
