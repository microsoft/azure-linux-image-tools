# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

BINARY_NAME := bin/imagecustomizerschemacli
SCHEMA_FILE :="../imagecustomizerapi/schema.json"

.PHONY: all
all: run

.PHONY: run
run:
	@echo "Generating $(SCHEMA_FILE)"
	go run . -o $(SCHEMA_FILE)
