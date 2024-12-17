// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package diskutils

import (
	"strconv"
	"strings"
)

// When calling mkfs, the default options change depending on the host OS you are running on and typically match
// what the distro has decided is best for their OS. For example, for ext2/3/4, the defaults are stored in
// /etc/mke2fs.conf.
// However, when building Azure Linux images, the defaults should be as consistent as possible and should only contain
// features that are supported on Azure Linux.

type Ext4Options struct {
	BlockSize int
	Features  []string
}

type FileSystemsOptions struct {
	Ext4 Ext4Options
}

var (
	Azl2Ext4Options = Ext4Options{
		BlockSize: 4096,
		Features: []string{"sparse_super", "large_file", "filetype", "resize_inode", "dir_index", "ext_attr",
			"has_journal", "extent", "huge_file", "flex_bg", "metadata_csum", "64bit", "dir_nlink", "extra_isize",
		},
	}

	Azl3Ext4Options = Ext4Options{
		BlockSize: 4096,
		Features: []string{"sparse_super", "large_file", "filetype", "resize_inode", "dir_index", "ext_attr",
			"has_journal", "extent", "huge_file", "flex_bg", "metadata_csum", "64bit", "dir_nlink", "extra_isize",
			"orphan_file",
		},
	}

	TargetOsFileSystemsOptions = map[TargetOs]FileSystemsOptions{
		TargetOsAzureLinux2: {
			Ext4: Azl2Ext4Options,
		},
		TargetOsAzureLinux3: {
			Ext4: Azl3Ext4Options,
		},
	}

	// A list of ext4 features and their minimum supported Linux kernel version.
	// Note: This list omits features that either:
	// - Are not used by one of the supported distros/versions, OR
	// - Are supported by MinKernelVersion (v5.4).
	// Ref: https://www.man7.org/linux/man-pages/man5/ext4.5.html
	Ext4FeaturesKernelSupport = map[string]Version{
		"orphan_file": {5, 15},
	}

	// A list of ext4 features and their minimum supported e2fsprogs versions.
	// Note: This list omits features that either:
	// - Are not used by one of the supported distros/versions, OR
	// - Are supported by MinKernelVersion (v5.4).
	// Ref: https://e2fsprogs.sourceforge.net/e2fsprogs-release.html
	Ext4FeaturesE2fsprogsSupport = map[string]Version{
		"orphan_file": {1, 47, 0},
	}

	MinKernelVersion = Version{5, 4}
)

func GetFileSystemOptions(hostKernelVersion Version, targetOs TargetOs, filesystemType string) []string {
	switch filesystemType {
	case "ext4":
		return GetExt4FileSystemOptions(hostKernelVersion, targetOs)

	case "xfs":
		return GetXfsFileSystemOptions(hostKernelVersion, targetOs)

	default:
		return []string(nil)
	}
}

func GetExt4FileSystemOptions(hostKernelVersion Version, targetOs TargetOs) []string {
	options := TargetOsFileSystemsOptions[targetOs]

	// Omit features not supported by the build host kernel.
	features := []string(nil)
	for _, feature := range options.Ext4.Features {
		requiredKernelVersion, hasRequiredKernelVersion := Ext4FeaturesKernelSupport[feature]
		if hasRequiredKernelVersion && requiredKernelVersion.Gt(hostKernelVersion) {
			// Feature is not supported on build host kernel.
			continue
		}

		features = append(features, feature)
	}

	// "none" requests no default options.
	featuresArg := []string{"none"}
	featuresArg = append(featuresArg, features...)

	args := []string{"-b", strconv.Itoa(options.Ext4.BlockSize), "-o"}
	args = append(args, strings.Join(featuresArg, ","))
	return args
}

func GetXfsFileSystemOptions(hostKernelVersion Version, targetOs TargetOs) []string {
	return []string(nil)
}
