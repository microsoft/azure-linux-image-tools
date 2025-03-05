//go:build tools

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package tools

// Prior to Go v1.24, this is the recommended way of tracking tool dependencies.
// https://go.dev/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
import (
	_ "github.com/google/go-licenses"
)
