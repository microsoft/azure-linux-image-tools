// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type Script struct {
	// Path is the path of the script file.
	// Mutually exclusive with 'Content'.
	Path string `yaml:"path" json:"path,omitempty"`
	// Content is the inline string content of the script to run.
	// Mutually exclusive with 'Path'.
	Content string `yaml:"content" json:"content,omitempty"`
	// Interpreter is the name or path of the process that will execute the script.
	// When 'Content' is specified, the default value is '/bin/sh'.
	// When 'Path' is specified and 'interpreter' is not specified, then the script file is executed directly.
	Interpreter string `yaml:"interpreter" json:"interpreter,omitempty"`
	// Arguments is a list of additional arguments to pass to the script.
	Arguments []string `yaml:"arguments" json:"arguments,omitempty"`
	// EnvironmentVariables are a set of environment variables to set when executing the script.
	EnvironmentVariables map[string]string `yaml:"environmentVariables" json:"environmentVariables,omitempty"`
	// Name is an optional value used to reference the script in the logs.
	Name string `yaml:"name" json:"name,omitempty"`

	SHA256Hash string `yaml:"sha256hash" json:"sha256hash,omitempty"`
}

func (s *Script) IsValid() error {
	if s.Path == "" && s.Content == "" {
		return fmt.Errorf("either path or content must have a value")
	}
	if s.Path != "" && s.Content != "" {
		return fmt.Errorf("path and content may not both have a value")
	}

	return nil
}
