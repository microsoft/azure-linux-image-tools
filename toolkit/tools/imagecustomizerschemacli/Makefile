# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

BINARY_NAME := bin/imagecustomizerschemacli
GO_FILES := $(wildcard *.go)
SCHEMA_FILE := schema.json

.PHONY: all
all: run

.PHONY: run
run:
	@echo "Generating $(SCHEMA_FILE)"
	go run . -o $(SCHEMA_FILE)
