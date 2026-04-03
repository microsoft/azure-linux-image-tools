// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/alecthomas/kong"
	"github.com/invopop/jsonschema"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
)

type RootCmd struct {
	OutputFile string `name:"output" short:"o" help:"Path to the output JSON schema file" required:""`
}

func main() {
	cli := &RootCmd{}
	_ = kong.Parse(cli,
		kong.Name("jsonschemacli"),
		kong.Description("A CLI tool to generate JSON schema for the image customizer API."),
		kong.HelpOptions{
			Compact:   true,
			FlagsLast: true,
		},
		kong.UsageOnError())

	if err := generateJSONSchema(cli.OutputFile); err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Printf("JSON schema has been written to %s\n", cli.OutputFile)
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
