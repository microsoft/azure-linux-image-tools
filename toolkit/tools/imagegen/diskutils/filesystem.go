// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package diskutils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/kernelversion"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/sliceutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/targetos"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/version"
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

type xfsOptions struct {
	Features []string
}

type fileSystemsOptions struct {
	Ext4 ext4Options
	Xfs  xfsOptions
}

var (
	// The default ext4 options used by an Azure Linux 2.0 image.
	// See, the /etc/mke2fs.conf file in an Azure Linux 2.0 image.
	azl2Ext4Options = ext4Options{
		BlockSize: 4096,
		Features: []string{"sparse_super", "large_file", "filetype", "resize_inode", "dir_index", "ext_attr",
			"has_journal", "extent", "huge_file", "flex_bg", "metadata_csum", "64bit", "dir_nlink", "extra_isize",
		},
	}

	// The default ext4 options used by an Azure Linux 3.0 image.
	// See, the /etc/mke2fs.conf file in an Azure Linux 3.0 image.
	azl3Ext4Options = ext4Options{
		BlockSize: 4096,
		Features: []string{"sparse_super", "large_file", "filetype", "resize_inode", "dir_index", "ext_attr",
			"has_journal", "extent", "huge_file", "flex_bg", "metadata_csum", "64bit", "dir_nlink", "extra_isize",
			"orphan_file",
		},
	}

	// The default xfs options used by an Azure Linux 2.0 image (kernel v5.15).
	// See, the /usr/share/xfsprogs/mkfs/lts_5.15.conf file.
	azl2XfsOptions = xfsOptions{
		Features: []string{"bigtime", "crc", "finobt", "inobtcount", "reflink", "rmapbt", "sparse"},
	}

	// The default xfs options used by an Azure Linux 3.0 image (kernel v6.6)
	// See, the /usr/share/xfsprogs/mkfs/lts_6.6.conf file.
	azl3XfsOptions = xfsOptions{
		Features: []string{"bigtime", "crc", "finobt", "inobtcount", "reflink", "rmapbt", "sparse", "nrext64"},
	}

	targetOsFileSystemsOptions = map[targetos.TargetOs]fileSystemsOptions{
		targetos.TargetOsAzureLinux2: {
			Ext4: azl2Ext4Options,
			Xfs:  azl2XfsOptions,
		},
		targetos.TargetOsAzureLinux3: {
			Ext4: azl3Ext4Options,
			Xfs:  azl3XfsOptions,
		},
	}

	// A list of ext4 features and their minimum supported Linux kernel version.
	//
	// Note: This list omits features that either:
	// - Are not used by one of the supported distros/versions, OR
	// - Are supported by MinKernelVersion (v5.4).
	//
	// Ref: https://www.man7.org/linux/man-pages/man5/ext4.5.html
	ext4FeaturesKernelSupport = map[string]version.Version{
		"orphan_file": {5, 15},
	}

	// A list of ext4 features and their minimum supported e2fsprogs versions.
	//
	// Note: This list omits features that either:
	// - Are not used by one of the supported distros/versions, OR
	// - Are supported by MinKernelVersion (v5.4).
	//
	// Ref: https://e2fsprogs.sourceforge.net/e2fsprogs-release.html
	ext4FeaturesE2fsprogsSupport = map[string]version.Version{
		"orphan_file": {1, 47, 0},
	}

	// A list of XFS features and their minimum supported xfsprogs / kernel versions.
	//
	// Note: XFS tools are developed in the Linux kernel tree. Hence, the kernel and xfsprogs versions are tied
	// together.
	xfsFeaturesSupport = map[string]version.Version{
		"bigtime":    {5, 10},
		"crc":        {3, 2, 0},
		"finobt":     {3, 2, 1},
		"inobtcount": {5, 10},
		"reflink":    {4, 10},
		"rmapbt":     {4, 10},
		"sparse":     {4, 10},
		"nrext64":    {5, 19},
	}

	// The mkfs.xfs flag each feature sits under.
	xfsFeatureFlag = map[string]string{
		"bigtime":    "metadata",
		"crc":        "metadata",
		"finobt":     "metadata",
		"inobtcount": "metadata",
		"reflink":    "metadata",
		"rmapbt":     "metadata",
		"sparse":     "inode",
		"nrext64":    "inode",
	}

	// The maximum version of mkfs.xfs that is currently supported.
	// This is used to prevent issues with newer versions of mkfs.xfs default enabling new features.
	maxMkfsXfsVersion = version.Version{6, 9}

	// The minmum supported kernel version. This helps avoid versions complexity for features that are old and therefore
	// basically universal.
	//
	// Relevant kernel versions:
	// - Ubuntu 22.04: v5.15
	// - Mariner 2.0: v5.15
	// - Mariner 3.0: v6.6
	minKernelVersion = version.Version{5, 4}

	// For exampke: mke2fs 1.47.0 (5-Feb-2023)
	mke2fsVersionRegex = regexp.MustCompile(`(?m)^mke2fs (\d+)\.(\d+)\.(\d+) \(\d+-[a-zA-Z]+-\d+\)$`)

	// For example: mkfs.xfs version 6.5.0
	mkfsXfsVersionRegex = regexp.MustCompile(`^mkfs\.xfs version (\d+)\.(\d+)\.(\d+)$`)
)

func getFileSystemOptions(targetOs targetos.TargetOs, filesystemType string) ([]string, error) {
	hostKernelVersion, err := kernelversion.GetBuildHostKernelVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get host kernel version:\n%w", err)
	}

	if minKernelVersion.Gt(hostKernelVersion) {
		return nil, fmt.Errorf("host kernel version (%s) is too old (min: %s)", minKernelVersion, hostKernelVersion)
	}

	options, hasOptions := targetOsFileSystemsOptions[targetOs]
	if !hasOptions {
		return nil, fmt.Errorf("unknown target OS (%s)", targetOs)
	}

	switch filesystemType {
	case "ext4":
		options, err := getExt4FileSystemOptions(hostKernelVersion, options)
		if err != nil {
			return nil, err
		}

		return options, nil

	case "xfs":
		options, err := getXfsFileSystemOptions(hostKernelVersion, options)
		if err != nil {
			return nil, err
		}

		return options, nil

	default:
		return []string(nil), nil
	}
}

func getExt4FileSystemOptions(hostKernelVersion version.Version, options fileSystemsOptions) ([]string, error) {
	mke2fsVersion, err := getMke2fsVersion()
	if err != nil {
		return nil, err
	}

	// "none" requests no default options.
	features := []string{"none"}

	for _, feature := range options.Ext4.Features {
		requiredKernelVersion, hasRequiredKernelVersion := ext4FeaturesKernelSupport[feature]
		if hasRequiredKernelVersion && requiredKernelVersion.Gt(hostKernelVersion) {
			// Feature is not supported on build host kernel.
			logger.Log.Infof("Build host kernel does not support ext4 feature (%s)", feature)
			continue
		}

		requiredMke2fsVersion, hasRequiredMke2fsVersion := ext4FeaturesE2fsprogsSupport[feature]
		if hasRequiredMke2fsVersion && requiredMke2fsVersion.Gt(mke2fsVersion) {
			// Feature is not supported by version of mkfs.ext4.
			logger.Log.Infof("mkfs.ext4 does not support ext4 feature (%s)", feature)
			continue
		}

		features = append(features, feature)
	}

	featuresArg := strings.Join(features, ",")

	args := []string{"-b", strconv.Itoa(options.Ext4.BlockSize), "-O", featuresArg}
	return args, nil
}

func getXfsFileSystemOptions(hostKernelVersion version.Version, options fileSystemsOptions) ([]string, error) {
	mkfsXfsVersion, err := getMkfsXfsVersion()
	if err != nil {
		return nil, err
	}

	if mkfsXfsVersion.Gt(maxMkfsXfsVersion) {
		// New versions of mkfs.xfs might add new default-enabled features in the future.
		// So, block newer versions of mkfs.xfs until we have verified there aren't any new XFS features that need to
		// set in the CLI args.
		return nil, fmt.Errorf("mkfs.xfs version (%s) is too new (max: %s)", mkfsXfsVersion, maxMkfsXfsVersion)
	}

	metadataArgs := []string(nil)
	inodeArgs := []string(nil)

	// Unlike mkfs.ext4, mkfs.xfs doesn't have a mechanism to disable all features.
	// So, explictly set every feature flag.
	for feature, requiredVersion := range xfsFeaturesSupport {
		enableFeature := sliceutils.ContainsValue(options.Xfs.Features, feature)

		if requiredVersion.Gt(hostKernelVersion) {
			// Feature is not supported on build host kernel.
			if enableFeature {
				logger.Log.Infof("Build host kernel does not support xfs feature (%s)", feature)
				enableFeature = false
			}
		}

		if requiredVersion.Gt(mkfsXfsVersion) {
			// Feature is not supported by mkfs.xfs.
			if enableFeature {
				logger.Log.Infof("mkfs.xfs does not support xfs feature (%s)", feature)
			}

			// This version of mkfs.xfs will not recognize the CLI option.
			// So, don't include it.
			continue
		}

		enableArg := "0"
		if enableFeature {
			enableArg = "1"
		}
		featureArg := fmt.Sprintf("%s=%s", feature, enableArg)

		switch xfsFeatureFlag[feature] {
		case "metadata":
			metadataArgs = append(metadataArgs, featureArg)

		case "inode":
			inodeArgs = append(inodeArgs, featureArg)
		}
	}

	metadataArgValue := strings.Join(metadataArgs, ",")
	inodeArgValue := strings.Join(inodeArgs, ",")

	args := []string{"-m", metadataArgValue, "-i", inodeArgValue}
	return args, nil
}

// Get the version of mkfs.ext4
func getMke2fsVersion() (version.Version, error) {
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
	version := version.Version{major, minor, patch}
	return version, nil
}

// Get the version of mkfs.xfs
func getMkfsXfsVersion() (version.Version, error) {
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
	version := version.Version{major, minor, patch}
	return version, nil
}
