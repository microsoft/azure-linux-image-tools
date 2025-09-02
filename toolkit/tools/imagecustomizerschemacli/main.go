// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/invopop/jsonschema"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"gopkg.in/alecthomas/kingpin.v2"
)

func main() {
	app := kingpin.New("jsonschemacli", "A CLI tool to generate JSON schema for the image customizer API.")
	outputFile := app.Flag("output", "Path to the output JSON schema file").Short('o').Required().String()

	kingpin.MustParse(app.Parse(os.Args[1:]))

	if err := generateJSONSchema(*outputFile); err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Printf("JSON schema has been written to %s\n", *outputFile)
}

func generateJSONSchema(outputFile string) error {
	reflector := jsonschema.Reflector{
		RequiredFromJSONSchemaTags: true,
	}

	schema := reflector.Reflect(&imagecustomizerapi.Config{})
	schemaJSON, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}

	// Write schema to file
	if err := os.WriteFile(outputFile, schemaJSON, 0o644); err != nil {
		return fmt.Errorf("failed to write schema to file: %w", err)
	}

	return nil
}
