// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/invopop/jsonschema"
	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
)

func main() {
	if len(os.Args) < 1 {
		fmt.Println("Usage: jsonschemacli <output_file>")
		os.Exit(1)
	}

	reflector := jsonschema.Reflector{}
	reflector.RequiredFromJSONSchemaTags = true
	schema := reflector.Reflect(&imagecustomizerapi.Config{})

	schemaJSON, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal schema: %v", err)
	}

	outputFile := os.Args[1]
	err = os.WriteFile(outputFile, schemaJSON, 0644)
	if err != nil {
		log.Fatalf("Failed to write schema to file: %v", err)
	}

	fmt.Printf("JSON schema has been written to %s\n", outputFile)
}
