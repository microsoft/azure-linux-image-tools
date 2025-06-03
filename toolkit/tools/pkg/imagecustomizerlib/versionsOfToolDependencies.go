package imagecustomizerlib

import (
	"os/exec"
	"regexp"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/osinfo"
)

func LogVersionsOfToolDeps() {
	// Map of version flags with corresponding packages
	versionFlags := map[string][]string{
		"--version": {
			"qemu-img", "rpm", "dd", "lsblk", "losetup", "sfdisk", "udevadm",
			"flock", "blkid", "sed", "createrepo", "genisoimage", "mkfs",
			"fsck", "fatlabel", "zstd", "veritysetup", "grub-install", "objcopy",
		},
		"-version": {
			"mksquashfs",
		},
		"version": {
			"openssl",
		},
		"-V": {
			"mkfs.ext4", "mkfs.xfs", "e2fsck", "xfs_repair", "xfs_admin",
		},
		"": {
			"mkfs.vfat", "resize2fs", "tune2fs",
		},
	}

	// Get distro and version
	distro, version := osinfo.GetDistroAndVersion()
	logger.Log.Debugf("Host OS distro: %s", distro)
	logger.Log.Debugf("Host OS version: %s", version)

	// Get versions of packages
	logger.Log.Debugf("Host Tools:")
	for versionFlag, pkgList := range versionFlags {
		for _, pkg := range pkgList {
			version, err := getPackageVersion(pkg, versionFlag)
			if err != nil {
				logger.Log.Debugf("%s: not installed or error retrieving version", pkg)
			} else {
				logger.Log.Debugf("%s: %s", pkg, version)
			}
		}
	}
}

// Function to get the version of a package
func getPackageVersion(pkg string, versionFlagParameter string) (string, error) {
	var cmd *exec.Cmd
	var pkgVersion string

	cmd = exec.Command(pkg, versionFlagParameter)
	output, _ := cmd.CombinedOutput()
	outputLines := strings.Split(string(output), "\n")

	// If the package does not have a version parameter, we need extract the version from the full output
	if versionFlagParameter == "" {
		// Regular expression to match various version formats including num.num.num, num.num, and alphanumeric versions
		re := regexp.MustCompile(`\b\d+(\.\d+){1,3}(-\w+)?\b`)
		for _, line := range outputLines {
			if re.MatchString(line) {
				pkgVersion = line
			}
		}
	} else {
		// Packages with a version parameter will have the version outputted as the first line
		pkgVersion = strings.Split(string(output), "\n")[0]
	}

	return pkgVersion, nil
}
