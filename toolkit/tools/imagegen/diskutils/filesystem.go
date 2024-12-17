// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package diskutils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
)

// When calling mkfs, the default options change depending on the host OS you are running on and typically match
// what the distro has decided is best for their OS. For example, for ext2/3/4, the defaults are stored in
// /etc/mke2fs.conf.
// However, when building Azure Linux images, the defaults should be as consistent as possible and should only contain
// features that are supported on Azure Linux.

type ext4Options struct {
	BlockSize int
	Features  []string
}

type fileSystemsOptions struct {
	Ext4 ext4Options
}

var (
	azl2Ext4Options = ext4Options{
		BlockSize: 4096,
		Features: []string{"sparse_super", "large_file", "filetype", "resize_inode", "dir_index", "ext_attr",
			"has_journal", "extent", "huge_file", "flex_bg", "metadata_csum", "64bit", "dir_nlink", "extra_isize",
		},
	}

	azl3Ext4Options = ext4Options{
		BlockSize: 4096,
		Features: []string{"sparse_super", "large_file", "filetype", "resize_inode", "dir_index", "ext_attr",
			"has_journal", "extent", "huge_file", "flex_bg", "metadata_csum", "64bit", "dir_nlink", "extra_isize",
			"orphan_file",
		},
	}

	targetOsFileSystemsOptions = map[TargetOs]fileSystemsOptions{
		TargetOsAzureLinux2: {
			Ext4: azl2Ext4Options,
		},
		TargetOsAzureLinux3: {
			Ext4: azl3Ext4Options,
		},
	}

	// A list of ext4 features and their minimum supported Linux kernel version.
	// Note: This list omits features that either:
	// - Are not used by one of the supported distros/versions, OR
	// - Are supported by MinKernelVersion (v5.4).
	// Ref: https://www.man7.org/linux/man-pages/man5/ext4.5.html
	ext4FeaturesKernelSupport = map[string]Version{
		"orphan_file": {5, 15},
	}

	// A list of ext4 features and their minimum supported e2fsprogs versions.
	// Note: This list omits features that either:
	// - Are not used by one of the supported distros/versions, OR
	// - Are supported by MinKernelVersion (v5.4).
	// Ref: https://e2fsprogs.sourceforge.net/e2fsprogs-release.html
	ext4FeaturesE2fsprogsSupport = map[string]Version{
		"orphan_file": {1, 47, 0},
	}

	minKernelVersion = Version{5, 4}

	// For exampke: mke2fs 1.47.0 (5-Feb-2023)
	mke2fsVersionRegex = regexp.MustCompile(`(?m)^mke2fs (\d+)\.(\d+)\.(\d+) \(\d+-[a-zA-Z]+-\d+\)$`)

	// For example: mkfs.xfs version 6.5.0
	mkfsXfsVersionRegex = regexp.MustCompile(`^mkfs\.xfs version (\d+)\.(\d+)\.(\d+)$`)
)

func getFileSystemOptions(targetOs TargetOs, filesystemType string) ([]string, error) {
	hostKernelVersion, err := GetBuildHostKernelVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get host kernel version:\n%w", err)
	}

	switch filesystemType {
	case "ext4":
		options, err := getExt4FileSystemOptions(hostKernelVersion, targetOs)
		if err != nil {
			return nil, err
		}

		return options, nil

	case "xfs":
		options, err := getXfsFileSystemOptions(hostKernelVersion, targetOs)
		if err != nil {
			return nil, err
		}

		return options, nil

	default:
		return []string(nil), nil
	}
}

func getExt4FileSystemOptions(hostKernelVersion Version, targetOs TargetOs) ([]string, error) {
	options := targetOsFileSystemsOptions[targetOs]

	mke2fsVersion, err := getMke2fsVersion()
	if err != nil {
		return nil, err
	}

	// Omit features not supported by the build host kernel.
	features := []string(nil)
	for _, feature := range options.Ext4.Features {
		requiredKernelVersion, hasRequiredKernelVersion := ext4FeaturesKernelSupport[feature]
		if hasRequiredKernelVersion && requiredKernelVersion.Gt(hostKernelVersion) {
			// Feature is not supported on build host kernel.
			logger.Log.Infof("Build host kernel does not support ext4 feature (%s)", feature)
			continue
		}

		requiredMke2fsVersion, hasRequiredMke2fsVersion := ext4FeaturesE2fsprogsSupport[feature]
		if hasRequiredMke2fsVersion && requiredMke2fsVersion.Gt(mke2fsVersion) {
			// Feature is not supported by mkfs.ext4.
			logger.Log.Infof("mkfs.ext4 does not support ext4 feature (%s)", feature)
			continue
		}

		features = append(features, feature)
	}

	// "none" requests no default options.
	featuresArg := []string{"none"}
	featuresArg = append(featuresArg, features...)

	args := []string{"-b", strconv.Itoa(options.Ext4.BlockSize), "-o"}
	args = append(args, strings.Join(featuresArg, ","))
	return args, nil
}

func getXfsFileSystemOptions(hostKernelVersion Version, targetOs TargetOs) ([]string, error) {
	return []string(nil), nil
}

func getMke2fsVersion() (Version, error) {
	_, stderr, err := shell.Execute("mke2fs", "-V")
	if err != nil {
		return nil, fmt.Errorf("failed to get mke2fs's version:\n%w", err)
	}

	fullVersionString := strings.TrimSpace(stderr)

	match := mke2fsVersionRegex.FindStringSubmatch(fullVersionString)
	if match == nil {
		return nil, fmt.Errorf("failed to parse mke2fs's version (%s)", fullVersionString)
	}

	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	patch, _ := strconv.Atoi(match[3])
	version := Version{major, minor, patch}
	return version, nil
}

func getMkfsXfsVersion() (Version, error) {
	stdout, _, err := shell.Execute("mkfs.xfs", "-V")
	if err != nil {
		return nil, fmt.Errorf("failed to get mkfs.xfs's version:\n%w", err)
	}

	fullVersionString := strings.TrimSpace(stdout)

	match := mkfsXfsVersionRegex.FindStringSubmatch(fullVersionString)
	if match == nil {
		return nil, fmt.Errorf("failed to parse mkfs.xfs's version (%s)", fullVersionString)
	}

	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	patch, _ := strconv.Atoi(match[3])
	version := Version{major, minor, patch}
	return version, nil
}
