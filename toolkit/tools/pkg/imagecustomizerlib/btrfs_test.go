// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSortBtrfsSubvolumesByDepth_EmptyInput_Pass(t *testing.T) {
	input := []btrfsSubvolumeConfig{}
	sorted := sortBtrfsSubvolumesByDepth(input)
	assert.Equal(t, 0, len(sorted))
}

func TestSortBtrfsSubvolumesByDepth_SingleElement_Pass(t *testing.T) {
	input := []btrfsSubvolumeConfig{{Path: "root"}}
	sorted := sortBtrfsSubvolumesByDepth(input)
	assert.Equal(t, input, sorted)
}

func TestSortBtrfsSubvolumesByDepth_UnsortedInput_Pass(t *testing.T) {
	input := []btrfsSubvolumeConfig{
		{Path: "root/var/lib/postgresql"},
		{Path: "root"},
		{Path: "home/user/documents/work"},
		{Path: "root/var"},
		{Path: "home"},
		{Path: "var/log"},
		{Path: "home/user"},
		{Path: "root/var/lib"},
	}

	// Sorted alphabetically by path. This ensures parent subvolumes are created before their children.
	expected := []btrfsSubvolumeConfig{
		{Path: "home"},
		{Path: "home/user"},
		{Path: "home/user/documents/work"},
		{Path: "root"},
		{Path: "root/var"},
		{Path: "root/var/lib"},
		{Path: "root/var/lib/postgresql"},
		{Path: "var/log"},
	}

	sorted := sortBtrfsSubvolumesByDepth(input)
	assert.Equal(t, expected, sorted)
}
