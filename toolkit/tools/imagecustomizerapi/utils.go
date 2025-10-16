// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"bytes"
	"fmt"
	"os"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/sliceutils"
	"gopkg.in/yaml.v3"
)

type HasIsValid interface {
	IsValid() error
}

func UnmarshalAndValidateYamlFile[ValueType HasIsValid](yamlFilePath string, value ValueType) error {
	var err error

	yamlFile, err := os.ReadFile(yamlFilePath)
	if err != nil {
		return err
	}

	err = UnmarshalAndValidateYaml(yamlFile, value)
	if err != nil {
		return err
	}

	return nil
}

func UnmarshalYamlFile[ValueType HasIsValid](yamlFilePath string, value ValueType) error {
	var err error

	yamlFile, err := os.ReadFile(yamlFilePath)
	if err != nil {
		return err
	}

	err = UnmarshalYaml(yamlFile, value)
	if err != nil {
		return err
	}

	return nil
}

func UnmarshalAndValidateYaml[ValueType HasIsValid](yamlData []byte, value ValueType) error {
	var err error

	err = UnmarshalYaml(yamlData, value)
	if err != nil {
		return err
	}

	err = value.IsValid()
	if err != nil {
		return err
	}

	return nil
}

func UnmarshalYaml[ValueType any](yamlData []byte, value ValueType) error {
	var err error

	reader := bytes.NewReader(yamlData)
	decoder := yaml.NewDecoder(reader)

	// Ensure unknown fields result in an error.
	decoder.KnownFields(true)

	err = decoder.Decode(value)
	if err != nil {
		return err
	}

	return nil
}

func MarshalYamlFile[ValueType any](yamlfilePath string, value ValueType) (err error) {
	yamlString, err := MarshalYaml(value)
	if err != nil {
		return err
	}

	file, err := os.Create(yamlfilePath)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := file.Close()
		if closeErr != nil {
			if err != nil {
				err = fmt.Errorf("%w:\nfailed to close (%s): %w", err, yamlfilePath, closeErr)
			} else {
				err = fmt.Errorf("failed to close (%s): %w", yamlfilePath, closeErr)
			}
		}
	}()

	_, err = file.WriteString(yamlString)
	if err != nil {
		return err
	}

	return nil
}

func MarshalYaml[ValueType any](value ValueType) (string, error) {
	yamlData, err := yaml.Marshal(value)
	if err != nil {
		return "", err
	}

	return string(yamlData), nil
}

// yaml.Node.Decode() doesn't respect the KnownFields() option.
// So, need to manually implement it.
func checkKnownFields(value *yaml.Node, structName string, validFields []string) error {
	for i := 0; i < len(value.Content); i += 2 {
		key := value.Content[i].Value
		if !sliceutils.ContainsValue(validFields, key) {
			return fmt.Errorf("line %d: field %s not found in type %s", value.Line, key, structName)
		}
	}

	return nil
}
