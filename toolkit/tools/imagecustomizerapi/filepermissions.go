// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/invopop/jsonschema"
	"gopkg.in/yaml.v3"
)

// The file permissions to set on the file.
//
// Accepted formats:
//
// - Octal string (e.g. "660")
type FilePermissions os.FileMode

func (p *FilePermissions) IsValid() error {
	// Check if there are set bits outside of the permissions bits.
	if *p & ^FilePermissions(os.ModePerm) != 0 {
		return fmt.Errorf("0o%o contains non-permission bits", *p)
	}

	return nil
}

func (p *FilePermissions) UnmarshalYAML(value *yaml.Node) error {
	var err error

	// Try to parse as a string.
	var strValue string
	err = value.Decode(&strValue)
	if err != nil {
		return fmt.Errorf("failed to parse filePermissions:\n%w", err)
	}

	// Try to parse the string as an octal number.
	fileModeUint, err := strconv.ParseUint(strValue, 8, 32)
	if err != nil {
		return fmt.Errorf("failed to parse filePermissions:\n%w", err)
	}

	*p = (FilePermissions)(fileModeUint)
	return nil
}

func (FilePermissions) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		OneOf: []*jsonschema.Schema{
			{
				Type:    "string",
				Pattern: "^[0-7]{3,4}$",
			},
			{
				Type:    "integer",
				Minimum: json.Number("0"),   // no negatives
				Maximum: json.Number("777"), // highest valid value
			},
		},
	}
}
