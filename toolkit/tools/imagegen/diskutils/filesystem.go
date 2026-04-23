// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package diskutils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/kernelversion"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/sliceutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/version"
)

// When calling mkfs, the default options change depending on the host OS you are running on and typically match
// what the distro has decided is best for their OS. For example, for ext2/3/4, the defaults are stored in
// /etc/mke2fs.conf.
// However, when building Azure Linux images, the defaults should be as consistent as possible and should only contain
// features that are supported on Azure Linux.

type btrfsOptions struct {
	Features []string
	Checksum string
}

type ext4Options struct {
	BlockSize int
	Features  []string
}

type xfsOptions struct {
	Features []string
}

type fileSystemsOptions struct {
	// The btrfs options to use.
	Btrfs btrfsOptions
	// The ext4 options to use.
	Ext4 ext4Options
	// The ext4 options to use for the partition that contains the /boot directory.
	BootExt4 ext4Options
	// The xfs options to use.
	Xfs xfsOptions
	// The xfs options to use for the partition that contains the /boot directory.
	BootXfs xfsOptions
}

var (
	// The default btrfs options used by an Azure Linux 2.0 image (kernel v5.15).
	// Features that are default-enabled as of btrfs-progs v5.15:
	// - extref (default since v3.12)
	// - skinny-metadata (default since v3.18)
	// - no-holes (default since v5.15)
	// - free-space-tree (default since v5.15)
	// Checksum default as of btrfs-progs v5.15: crc32c
	azl2BtrfsOptions = btrfsOptions{
		Features: []string{"extref", "skinny-metadata", "no-holes", "free-space-tree"},
		Checksum: "crc32c",
	}

	// The default btrfs options used by an Azure Linux 3.0 image (kernel v6.6).
	// Same as AZL2 since nothing has changed since v5.15.
	azl3BtrfsOptions = btrfsOptions{
		Features: []string{"extref", "skinny-metadata", "no-holes", "free-space-tree"},
		Checksum: "crc32c",
	}

	// The default btrfs options used by an Azure Linux 4.0 image (kernel v6.18).
	// Same as AZL3 since nothing has changed since v5.15.
	azl4BtrfsOptions = btrfsOptions{
		Features: []string{"extref", "skinny-metadata", "no-holes", "free-space-tree"},
		Checksum: "crc32c",
	}

	// The default btrfs options used by Fedora 42 (kernel v6.11+)
	fedora42BtrfsOptions = btrfsOptions{
		Features: []string{"extref", "skinny-metadata", "no-holes", "free-space-tree"},
		Checksum: "crc32c",
	}

	// The default btrfs options used by an Ubuntu 22.04 image.
	// Features that are default-enabled as of btrfs-progs v5.16.2 (see `mkfs.btrfs -O list-all`):
	// - extref (default since v3.12)
	// - skinny-metadata (default since v3.18)
	// - no-holes (default since v5.15)
	// Note: free-space-tree is NOT default-enabled in Ubuntu 22.04's btrfs-progs v5.16.2.
	// Checksum default as of btrfs-progs v5.16.2: crc32c
	ubuntu2204BtrfsOptions = btrfsOptions{
		Features: []string{"extref", "skinny-metadata", "no-holes"},
		Checksum: "crc32c",
	}

	// The default btrfs options used by an Ubuntu 24.04 image.
	// Features that are default-enabled as of btrfs-progs v6.6.3 (see `mkfs.btrfs -O list-all`):
	// - extref (default since v3.12)
	// - skinny-metadata (default since v3.18)
	// - no-holes (default since v5.15)
	// - free-space-tree (default since v5.15)
	// Checksum default as of btrfs-progs v6.6.3: crc32c
	ubuntu2404BtrfsOptions = btrfsOptions{
		Features: []string{"extref", "skinny-metadata", "no-holes", "free-space-tree"},
		Checksum: "crc32c",
	}

	// The default ext4 options used by an Azure Linux 2.0 image.
	// See, the /etc/mke2fs.conf file in an Azure Linux 2.0 image.
	azl2Ext4Options = ext4Options{
		BlockSize: 4096,
		Features: []string{
			"sparse_super", "large_file", "filetype", "resize_inode", "dir_index", "ext_attr",
			"has_journal", "extent", "huge_file", "flex_bg", "metadata_csum", "64bit", "dir_nlink", "extra_isize",
		},
	}

	// The default ext4 options used by an Azure Linux 3.0 image.
	// See, the /etc/mke2fs.conf file in an Azure Linux 3.0 image.
	azl3Ext4Options = ext4Options{
		BlockSize: 4096,
		Features: []string{
			"sparse_super", "large_file", "filetype", "resize_inode", "dir_index", "ext_attr",
			"has_journal", "extent", "huge_file", "flex_bg", "metadata_csum", "64bit", "dir_nlink", "extra_isize",
			"orphan_file",
		},
	}

	// The default ext4 options used by an Azure Linux 4.0 image.
	// See, the /etc/mke2fs.conf file in an Azure Linux 4.0 image.
	azl4Ext4Options = ext4Options{
		BlockSize: 4096,
		Features: []string{
			"sparse_super", "large_file", "filetype", "resize_inode", "dir_index", "ext_attr", "has_journal", "extent",
			"huge_file", "flex_bg", "metadata_csum", "metadata_csum_seed", "64bit", "dir_nlink", "extra_isize",
			"orphan_file",
		},
	}

	// GRUB 2.06 doesn't support 'metadata_csum_seed'.
	azl4BootExt4Options = ext4Options{
		BlockSize: 4096,
		Features: []string{
			"sparse_super", "large_file", "filetype", "resize_inode", "dir_index", "ext_attr", "has_journal", "extent",
			"huge_file", "flex_bg", "metadata_csum", "64bit", "dir_nlink", "extra_isize",
			"orphan_file",
		},
	}

	// The default ext4 options used by Fedora 42 (kernel v6.11+)
	// Based on typical Fedora defaults with modern ext4 features
	fedora42Ext4Options = ext4Options{
		BlockSize: 4096,
		Features: []string{
			"sparse_super", "large_file", "filetype", "resize_inode", "dir_index", "ext_attr",
			"has_journal", "extent", "huge_file", "flex_bg", "metadata_csum", "64bit", "dir_nlink", "extra_isize",
			"orphan_file",
		},
	}

	// The default ext4 options used by an Ubuntu 22.04 image.
	// See, the /etc/mke2fs.conf file in an Ubuntu 22.04 image (e2fsprogs v1.46.5).
	// Note: orphan_file is NOT supported (requires e2fsprogs >= 1.47.0).
	ubuntu2204Ext4Options = ext4Options{
		BlockSize: 4096,
		Features: []string{
			"sparse_super", "large_file", "filetype", "resize_inode", "dir_index", "ext_attr",
			"has_journal", "extent", "huge_file", "flex_bg", "metadata_csum", "64bit", "dir_nlink", "extra_isize",
		},
	}

	// The default ext4 options used by an Ubuntu 24.04 image.
	// See, the /etc/mke2fs.conf file in an Ubuntu 24.04 image (e2fsprogs v1.47.0).
	// Note: orphan_file is NOT in Ubuntu 24.04's mke2fs.conf default features,
	// even though e2fsprogs v1.47.0 supports it.
	ubuntu2404Ext4Options = ext4Options{
		BlockSize: 4096,
		Features: []string{
			"sparse_super", "large_file", "filetype", "resize_inode", "dir_index", "ext_attr",
			"has_journal", "extent", "huge_file", "flex_bg", "metadata_csum", "64bit", "dir_nlink", "extra_isize",
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

	// GRUB 2.06 doesn't support 'nrext64'.
	azl3BootXfsOptions = xfsOptions{
		Features: []string{"bigtime", "crc", "finobt", "inobtcount", "reflink", "rmapbt", "sparse"},
	}

	// The default xfs options used by an Azure Linux 4.0 image (kernel v6.18).
	// See, the /usr/share/xfsprogs/mkfs/lts_6.12.conf file.
	azl4XfsOptions = xfsOptions{
		Features: []string{"bigtime", "crc", "finobt", "inobtcount", "reflink", "rmapbt", "sparse", "nrext64"},
	}

	// GRUB 2.12 supports 'nrext64'.
	azl4BootXfsOptions = xfsOptions{
		Features: []string{"bigtime", "crc", "finobt", "inobtcount", "reflink", "rmapbt", "sparse", "nrext64"},
	}

	// The default xfs options used by Fedora 42 (kernel v6.11+)
	// Based on modern XFS features supported in recent kernels
	fedora42XfsOptions = xfsOptions{
		Features: []string{"bigtime", "crc", "finobt", "inobtcount", "reflink", "rmapbt", "sparse", "nrext64"},
	}

	// The default xfs options used by an Ubuntu 22.04 image (xfsprogs v5.13.0).
	// Ubuntu 22.04's xfsprogs v5.13.0 has no /usr/share/xfsprogs/mkfs/ config directory,
	// so the compiled-in defaults are the only source. See, `mkfs.xfs -N` dry-run output.
	// Note: bigtime, rmapbt, and inobtcount are NOT default-enabled in xfsprogs v5.13.0.
	ubuntu2204XfsOptions = xfsOptions{
		Features: []string{"crc", "finobt", "reflink", "sparse"},
	}

	// The default xfs options used by an Ubuntu 24.04 image (xfsprogs v6.6.0).
	// Ubuntu 24.04's xfsprogs v6.6.0 ships /usr/share/xfsprogs/mkfs/lts_*.conf files, but these are
	// named configuration profiles (used via `mkfs.xfs -c options=<file>`), NOT automatically loaded
	// defaults. The actual defaults are compiled into the binary. See, `mkfs.xfs -N` dry-run output.
	// Note: Ubuntu 24.04 reverts the upstream nrext64 default via a Debian patch
	// (LP: #2044623), so nrext64 is NOT included here.
	ubuntu2404XfsOptions = xfsOptions{
		Features: []string{"bigtime", "crc", "finobt", "inobtcount", "reflink", "rmapbt", "sparse"},
	}

	targetOsFileSystemsOptions = map[targetos.TargetOs]fileSystemsOptions{
		targetos.TargetOsAzureLinux2: {
			Btrfs:    azl2BtrfsOptions,
			Ext4:     azl2Ext4Options,
			BootExt4: azl2Ext4Options,
			Xfs:      azl2XfsOptions,
			BootXfs:  azl2XfsOptions,
		},
		targetos.TargetOsAzureLinux3: {
			Btrfs:    azl3BtrfsOptions,
			Ext4:     azl3Ext4Options,
			BootExt4: azl3Ext4Options,
			Xfs:      azl3XfsOptions,
			BootXfs:  azl3BootXfsOptions,
		},
		targetos.TargetOsAzureLinux4: {
			Btrfs:    azl4BtrfsOptions,
			Ext4:     azl4Ext4Options,
			BootExt4: azl4BootExt4Options,
			Xfs:      azl4XfsOptions,
			BootXfs:  azl4BootXfsOptions,
		},
		targetos.TargetOsFedora42: {
			Btrfs:    fedora42BtrfsOptions,
			Ext4:     fedora42Ext4Options,
			BootExt4: fedora42Ext4Options,
			Xfs:      fedora42XfsOptions,
			BootXfs:  fedora42XfsOptions,
		},
		targetos.TargetOsUbuntu2204: {
			Btrfs:    ubuntu2204BtrfsOptions,
			Ext4:     ubuntu2204Ext4Options,
			BootExt4: ubuntu2204Ext4Options,
			Xfs:      ubuntu2204XfsOptions,
			BootXfs:  ubuntu2204XfsOptions,
		},
		targetos.TargetOsUbuntu2404: {
			Btrfs:    ubuntu2404BtrfsOptions,
			Ext4:     ubuntu2404Ext4Options,
			BootExt4: ubuntu2404Ext4Options,
			Xfs:      ubuntu2404XfsOptions,
			BootXfs:  ubuntu2404XfsOptions,
		},
	}

	// A list of btrfs features and their minimum supported kernel versions.
	//
	// Note: This list omits features that either:
	// - Are not used by one of the supported distros/versions, OR
	// - Are supported by MinKernelVersion (v5.4).
	//
	// All btrfs features (as of btrfs-progs 6.17):
	// - mixed-bg (since kernel v2.6.37)
	// - extref (v3.7)
	// - raid56 (v3.9)
	// - skinny-metadata (v3.10)
	// - no-holes (v3.14)
	// - zoned (v5.12)
	// - quota (v3.4)
	// - free-space-tree (v4.5)
	// - block-group-tree (v6.1)
	// - raid-stripe-tree (v6.7)
	// - squota (v6.7)
	//
	// Ref: https://btrfs.readthedocs.io/en/latest/mkfs.btrfs.html
	btrfsFeaturesKernelSupport = map[string]version.Version{
		"extref":           {3, 7},
		"skinny-metadata":  {3, 10},
		"no-holes":         {3, 14},
		"free-space-tree":  {4, 5},
		"block-group-tree": {6, 1},
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
		"orphan_file":        {1, 47, 0},
		"metadata_csum_seed": {1, 47, 0},
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
		"metadir":    {6, 13},
		"reflink":    {4, 10},
		"rmapbt":     {4, 10},
		"autofsck":   {6, 10},
		"sparse":     {4, 10},
		"nrext64":    {5, 19},
		"exchange":   {6, 10},
		"parent":     {6, 10},
	}

	// The mkfs.xfs flag each feature sits under.
	xfsFeatureFlag = map[string]string{
		"bigtime":    "metadata",
		"crc":        "metadata",
		"finobt":     "metadata",
		"inobtcount": "metadata",
		"metadir":    "metadata",
		"reflink":    "metadata",
		"rmapbt":     "metadata",
		"autofsck":   "metadata",
		"sparse":     "inode",
		"nrext64":    "inode",
		"exchange":   "inode",
		"parent":     "naming",
	}

	// The maximum version of mkfs.btrfs that is currently supported.
	// This is used to prevent issues with newer versions of mkfs.btrfs default enabling new features.
	// Ref: https://btrfs.readthedocs.io/en/latest/CHANGES.html
	maxMkfsBtrfsVersion = version.Version{6, 19, 1}

	// The maximum version of mkfs.xfs that is currently supported.
	// This is used to prevent issues with newer versions of mkfs.xfs default enabling new features.
	maxMkfsXfsVersion = version.Version{6, 15}

	// The minimum supported kernel version. This helps avoid versions complexity for features that are old and therefore
	// basically universal.
	//
	// Relevant kernel versions:
	// - Ubuntu 22.04: v5.15
	// - Ubuntu 24.04: v6.8
	// - Mariner 2.0: v5.15
	// - Mariner 3.0: v6.6
	minKernelVersion = version.Version{5, 4}

	// For example: mkfs.btrfs, part of btrfs-progs v6.8
	mkfsBtrfsVersionRegex = regexp.MustCompile(`(?m)^mkfs\.btrfs, part of btrfs-progs v(\d+)\.(\d+)(?:\.(\d+))?$`)

	// For exampke: mke2fs 1.47.0 (5-Feb-2023)
	mke2fsVersionRegex = regexp.MustCompile(`(?m)^mke2fs (\d+)\.(\d+)\.(\d+) \(\d+-[a-zA-Z]+-\d+\)$`)

	// For example: mkfs.xfs version 6.5.0
	mkfsXfsVersionRegex = regexp.MustCompile(`^mkfs\.xfs version (\d+)\.(\d+)\.(\d+)$`)
)

// Params:
// - targetOs: The OS the filesystem is being created for.
// - filesystemType: The requested filesystem type.
// - isBootPartition: Will the partition contain the /boot directory?
func getFileSystemOptions(targetOs targetos.TargetOs, filesystemType string, isBootPartition bool) ([]string, error) {
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
	case "btrfs":
		options, err := getBtrfsFileSystemOptions(hostKernelVersion, options)
		if err != nil {
			return nil, err
		}

		return options, nil

	case "ext4":
		options, err := getExt4FileSystemOptions(hostKernelVersion, options, isBootPartition)
		if err != nil {
			return nil, err
		}

		return options, nil

	case "xfs":
		options, err := getXfsFileSystemOptions(hostKernelVersion, options, isBootPartition)
		if err != nil {
			return nil, err
		}

		return options, nil

	default:
		return []string(nil), nil
	}
}

func getBtrfsFileSystemOptions(hostKernelVersion version.Version, options fileSystemsOptions) ([]string, error) {
	mkfsBtrfsVersion, err := getMkfsBtrfsVersion()
	if err != nil {
		return nil, err
	}

	if mkfsBtrfsVersion.Gt(maxMkfsBtrfsVersion) {
		// New versions of mkfs.btrfs might add new default-enabled features in the future.
		// So, block newer versions of mkfs.btrfs until we have verified there aren't any new btrfs features that need
		// to be set in the CLI args.
		return nil, fmt.Errorf("mkfs.btrfs version (%s) is too new (max: %s)", mkfsBtrfsVersion, maxMkfsBtrfsVersion)
	}

	// Build list of features to enable.
	// Unlike ext4's "none" option, btrfs doesn't have a way to disable all defaults.
	// Instead, we explicitly list the features we want.
	enableFeatures := []string{}
	disableFeatures := []string{}

	// Go through all known features and explicitly enable or disable them based on the target OS options.
	for feature, requiredVersion := range btrfsFeaturesKernelSupport {
		enableFeature := sliceutils.ContainsValue(options.Btrfs.Features, feature)

		if requiredVersion.Gt(hostKernelVersion) {
			// Feature is not supported on build host kernel.
			if enableFeature {
				logger.Log.Infof("Build host kernel does not support btrfs feature (%s)", feature)
				enableFeature = false
			}
		}

		if requiredVersion.Gt(mkfsBtrfsVersion) {
			// Feature is not supported by mkfs.btrfs.
			if enableFeature {
				logger.Log.Infof("mkfs.btrfs does not support btrfs feature (%s)", feature)
			}

			continue
		}

		if enableFeature {
			enableFeatures = append(enableFeatures, feature)
		} else {
			disableFeatures = append(disableFeatures, "^"+feature)
		}
	}

	allFeatures := append(enableFeatures, disableFeatures...)

	args := []string{}

	if len(allFeatures) > 0 {
		featuresArg := strings.Join(allFeatures, ",")
		args = append(args, "-O", featuresArg)
	}

	if options.Btrfs.Checksum != "" {
		args = append(args, "--csum", options.Btrfs.Checksum)
	}

	return args, nil
}

func getExt4FileSystemOptions(hostKernelVersion version.Version, options fileSystemsOptions, isBootPartition bool,
) ([]string, error) {
	mke2fsVersion, err := getMke2fsVersion()
	if err != nil {
		return nil, err
	}

	ext4Options := options.Ext4
	if isBootPartition {
		ext4Options = options.BootExt4
	}

	// "none" requests no default options.
	features := []string{"none"}

	for _, feature := range ext4Options.Features {
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

	args := []string{"-b", strconv.Itoa(ext4Options.BlockSize), "-O", featuresArg}
	return args, nil
}

func getXfsFileSystemOptions(hostKernelVersion version.Version, options fileSystemsOptions, isBootPartition bool,
) ([]string, error) {
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

	xfsOptions := options.Xfs
	if isBootPartition {
		xfsOptions = options.BootXfs
	}

	metadataArgs := []string(nil)
	inodeArgs := []string(nil)
	namingArgs := []string(nil)

	// Unlike mkfs.ext4, mkfs.xfs doesn't have a mechanism to disable all features.
	// So, explictly set every feature flag.
	for feature, requiredVersion := range xfsFeaturesSupport {
		enableFeature := sliceutils.ContainsValue(xfsOptions.Features, feature)

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

		case "naming":
			namingArgs = append(namingArgs, featureArg)
		}
	}

	metadataArgValue := strings.Join(metadataArgs, ",")
	inodeArgValue := strings.Join(inodeArgs, ",")
	namingArgValue := strings.Join(namingArgs, ",")

	args := []string{"-m", metadataArgValue, "-i", inodeArgValue, "-n", namingArgValue}
	return args, nil
}

// Get the version of mkfs.btrfs
func getMkfsBtrfsVersion() (version.Version, error) {
	stdout, _, err := shell.Execute("mkfs.btrfs", "-V")
	if err != nil {
		return nil, fmt.Errorf("failed to get mkfs.btrfs's version:\n%w", err)
	}

	fullVersionString := strings.TrimSpace(stdout)

	match := mkfsBtrfsVersionRegex.FindStringSubmatch(fullVersionString)
	if match == nil {
		return nil, fmt.Errorf("failed to parse mkfs.btrfs's version (%s)", fullVersionString)
	}

	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	patch := 0
	if match[3] != "" {
		patch, _ = strconv.Atoi(match[3])
	}
	version := version.Version{major, minor, patch}
	return version, nil
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
