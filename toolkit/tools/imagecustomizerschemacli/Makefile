# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

BINARY_NAME := bin/imagecustomizerschemacli
GO_FILES := $(wildcard *.go)
SCHEMA_FILE := schema.json

all: build run

build: $(GO_FILES)
	mkdir -p bin
	go build -o $(BINARY_NAME) .

run: build
	@echo "Running $(BINARY_NAME) with $(SCHEMA_FILE)"
	./$(BINARY_NAME) $(SCHEMA_FILE)

clean:
	rm -rf bin

.PHONY: all build run clean
