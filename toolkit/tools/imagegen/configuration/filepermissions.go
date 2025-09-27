// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

//

package configuration

import (
	"os"
)

// The file permissions to set on the file.
//
// Accepted formats:
//
// - Octal string (e.g. "660")
type FilePermissions os.FileMode
