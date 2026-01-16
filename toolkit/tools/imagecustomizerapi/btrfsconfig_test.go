// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBtrfsConfigIsValid_EmptySubvolumes_Pass(t *testing.T) {
	config := BtrfsConfig{
		Subvolumes: []BtrfsSubvolume{},
	}
	err := config.IsValid()
	assert.NoError(t, err)
}

func TestBtrfsConfigIsValid_NilSubvolumes_Pass(t *testing.T) {
	config := BtrfsConfig{}
	err := config.IsValid()
	assert.NoError(t, err)
}

func TestBtrfsConfigIsValid_SingleValidSubvolume_Pass(t *testing.T) {
	config := BtrfsConfig{
		Subvolumes: []BtrfsSubvolume{
			{
				Path: "root",
			},
		},
	}
	err := config.IsValid()
	assert.NoError(t, err)
}

func TestBtrfsConfigIsValid_MultipleValidSubvolumes_Pass(t *testing.T) {
	config := BtrfsConfig{
		Subvolumes: []BtrfsSubvolume{
			{
				Path: "root",
				MountPoint: &MountPoint{
					Path: "/",
				},
			},
			{
				Path: "home",
				MountPoint: &MountPoint{
					Path: "/home",
				},
			},
			{
				Path: "var/log",
				MountPoint: &MountPoint{
					Path: "/var/log",
				},
			},
		},
	}
	err := config.IsValid()
	assert.NoError(t, err)
}

func TestBtrfsConfigIsValid_InvalidSubvolumeEmptyPath_Fail(t *testing.T) {
	config := BtrfsConfig{
		Subvolumes: []BtrfsSubvolume{
			{
				Path: "", // Invalid: empty
			},
		},
	}
	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid subvolume at index 0")
}

func TestBtrfsConfigIsValid_InvalidSubvolumeAtSecondIndex_Fail(t *testing.T) {
	config := BtrfsConfig{
		Subvolumes: []BtrfsSubvolume{
			{
				Path: "root",
			},
			{
				Path: "/invalid", // Invalid: starts with '/'
			},
		},
	}
	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid subvolume at index 1")
}

func TestBtrfsConfigIsValid_DuplicatePaths_Fail(t *testing.T) {
	config := BtrfsConfig{
		Subvolumes: []BtrfsSubvolume{
			{
				Path: "root",
			},
			{
				Path: "root",
			},
		},
	}
	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid subvolume at index 1")
	assert.ErrorContains(t, err, "duplicate path (root)")
}

func TestBtrfsConfigIsValid_DuplicatePathsNonConsecutive_Fail(t *testing.T) {
	config := BtrfsConfig{
		Subvolumes: []BtrfsSubvolume{
			{
				Path: "root",
			},
			{
				Path: "home",
			},
			{
				Path: "root",
			},
		},
	}
	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid subvolume at index 2")
	assert.ErrorContains(t, err, "duplicate path (root)")
}

func TestBtrfsConfigIsValid_MountPointLoop_Fail(t *testing.T) {
	// This creates a loop: subvol=/var mounted at /var/log, subvol=/var/log mounted at /var
	// Following paths: /var -> /var/log -> /var/log/log -> ... (infinite)
	config := BtrfsConfig{
		Subvolumes: []BtrfsSubvolume{
			{
				Path: "var",
				MountPoint: &MountPoint{
					Path: "/var/log",
				},
			},
			{
				Path: "var/log",
				MountPoint: &MountPoint{
					Path: "/var",
				},
			},
		},
	}
	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "subvolume mount point loop detected")
}

func TestBtrfsConfigIsValid_MountPointLoopReversedOrder_Fail(t *testing.T) {
	// Same loop, different order in config
	config := BtrfsConfig{
		Subvolumes: []BtrfsSubvolume{
			{
				Path: "var/log",
				MountPoint: &MountPoint{
					Path: "/var",
				},
			},
			{
				Path: "var",
				MountPoint: &MountPoint{
					Path: "/var/log",
				},
			},
		},
	}
	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "subvolume mount point loop detected")
}

func TestBtrfsConfigIsValid_MountPointLoopDeeplyNested_Fail(t *testing.T) {
	// Deeper nesting: subvol=/a/b/c mounted at /x, subvol=/a mounted at /x/y/z
	config := BtrfsConfig{
		Subvolumes: []BtrfsSubvolume{
			{
				Path: "a",
				MountPoint: &MountPoint{
					Path: "/x/y/z",
				},
			},
			{
				Path: "a/b/c",
				MountPoint: &MountPoint{
					Path: "/x",
				},
			},
		},
	}
	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "subvolume mount point loop detected")
}

func TestBtrfsConfigIsValid_MountPointLoopWithThreeSubvolumes_Fail(t *testing.T) {
	// Loop between two subvolumes, third is unrelated
	config := BtrfsConfig{
		Subvolumes: []BtrfsSubvolume{
			{
				Path: "root",
				MountPoint: &MountPoint{
					Path: "/",
				},
			},
			{
				Path: "var",
				MountPoint: &MountPoint{
					Path: "/var/log",
				},
			},
			{
				Path: "var/log",
				MountPoint: &MountPoint{
					Path: "/var",
				},
			},
		},
	}
	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "subvolume mount point loop detected")
}

func TestBtrfsConfigIsValid_NoLoopNestedPathsMatchingMounts_Pass(t *testing.T) {
	// Nested paths with matching nested mounts - no loop
	config := BtrfsConfig{
		Subvolumes: []BtrfsSubvolume{
			{
				Path: "var",
				MountPoint: &MountPoint{
					Path: "/var",
				},
			},
			{
				Path: "var/log",
				MountPoint: &MountPoint{
					Path: "/var/log",
				},
			},
		},
	}
	err := config.IsValid()
	assert.NoError(t, err)
}

func TestBtrfsConfigIsValid_NoLoopFlatSubvolumes_Pass(t *testing.T) {
	// Flat subvolumes with various mount points - no loop possible
	config := BtrfsConfig{
		Subvolumes: []BtrfsSubvolume{
			{
				Path: "root",
				MountPoint: &MountPoint{
					Path: "/",
				},
			},
			{
				Path: "home",
				MountPoint: &MountPoint{
					Path: "/home",
				},
			},
			{
				Path: "var",
				MountPoint: &MountPoint{
					Path: "/var",
				},
			},
		},
	}
	err := config.IsValid()
	assert.NoError(t, err)
}

func TestBtrfsConfigIsValid_NoLoopNoMountPoints_Pass(t *testing.T) {
	// Subvolumes without mount points can't create loops
	config := BtrfsConfig{
		Subvolumes: []BtrfsSubvolume{
			{
				Path: "var",
			},
			{
				Path: "var/log",
			},
		},
	}
	err := config.IsValid()
	assert.NoError(t, err)
}

func TestBtrfsConfigIsValid_NoLoopOneMountPoint_Pass(t *testing.T) {
	// Only one has a mount point, can't create loop
	config := BtrfsConfig{
		Subvolumes: []BtrfsSubvolume{
			{
				Path: "var",
				MountPoint: &MountPoint{
					Path: "/var/log",
				},
			},
			{
				Path: "var/log",
			},
		},
	}
	err := config.IsValid()
	assert.NoError(t, err)
}

func TestBtrfsConfigIsValid_MountPointLoopWithDotDot_Fail(t *testing.T) {
	// Loop detected even when mount point uses ..
	config := BtrfsConfig{
		Subvolumes: []BtrfsSubvolume{
			{
				Path: "var",
				MountPoint: &MountPoint{
					Path: "/var/log/../log", // normalizes to /var/log
				},
			},
			{
				Path: "var/log",
				MountPoint: &MountPoint{
					Path: "/var",
				},
			},
		},
	}
	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "subvolume mount point loop detected")
}

func TestBtrfsConfigIsValid_MountPointLoopWithDot_Fail(t *testing.T) {
	// Loop detected even when mount point uses .
	config := BtrfsConfig{
		Subvolumes: []BtrfsSubvolume{
			{
				Path: "var",
				MountPoint: &MountPoint{
					Path: "/var/./log", // normalizes to /var/log
				},
			},
			{
				Path: "var/log",
				MountPoint: &MountPoint{
					Path: "/./var", // normalizes to /var
				},
			},
		},
	}
	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "subvolume mount point loop detected")
}

func TestBtrfsConfigIsValid_MountPointLoopWithDoubleSlash_Fail(t *testing.T) {
	// Loop detected even when mount point uses //
	config := BtrfsConfig{
		Subvolumes: []BtrfsSubvolume{
			{
				Path: "var",
				MountPoint: &MountPoint{
					Path: "/var//log", // normalizes to /var/log
				},
			},
			{
				Path: "var/log",
				MountPoint: &MountPoint{
					Path: "//var", // normalizes to /var
				},
			},
		},
	}
	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "subvolume mount point loop detected")
}

func TestBtrfsConfigIsValid_MountPointLoopWithTrailingSlash_Fail(t *testing.T) {
	// Loop detected even when mount point has trailing slash
	config := BtrfsConfig{
		Subvolumes: []BtrfsSubvolume{
			{
				Path: "var",
				MountPoint: &MountPoint{
					Path: "/var/log/", // normalizes to /var/log
				},
			},
			{
				Path: "var/log",
				MountPoint: &MountPoint{
					Path: "/var/", // normalizes to /var
				},
			},
		},
	}
	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "subvolume mount point loop detected")
}

func TestBtrfsConfigIsValid_NoLoopWithNormalizedPaths_Pass(t *testing.T) {
	// No loop after normalization - paths look different but normalize to same hierarchy
	config := BtrfsConfig{
		Subvolumes: []BtrfsSubvolume{
			{
				Path: "var",
				MountPoint: &MountPoint{
					Path: "/var/./", // normalizes to /var
				},
			},
			{
				Path: "var/log",
				MountPoint: &MountPoint{
					Path: "/var/../var/log//", // normalizes to /var/log
				},
			},
		},
	}
	err := config.IsValid()
	assert.NoError(t, err)
}
