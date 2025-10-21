// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package installutils

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/configuration"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/resources"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/userutils"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const (
	PackageManifestRelativePath = "image_pkg_manifest_installroot.json"

	// NullDevice represents the /dev/null device used as a mount device for overlay images.
	NullDevice = "/dev/null"

	// CmdlineSELinuxSecurityArg is the "security" arg needed for enabling SELinux.
	CmdlineSELinuxSecurityArg = "security=selinux"

	// CmdlineSELinuxEnabledArg is the "selinux" arg needed for disabling SELinux.
	CmdlineSELinuxDisabledArg = "selinux=0"

	// CmdlineSELinuxEnabledArg is the "selinux" arg needed for enabling SELinux.
	CmdlineSELinuxEnabledArg = "selinux=1"

	// CmdlineSELinuxEnforcingArg is the arg required for forcing SELinux to be in enforcing mode.
	CmdlineSELinuxEnforcingArg = "enforcing=1"

	// CmdlineSELinuxPermissiveArg is the arg for SELinux to be in force-permissive mode.
	CmdlineSELinuxPermissiveArg = "enforcing=0"

	// CmdlineSELinuxSettings is the kernel command-line args for enabling SELinux.
	CmdlineSELinuxSettings = CmdlineSELinuxSecurityArg + " " + CmdlineSELinuxEnabledArg

	// CmdlineSELinuxForceEnforcing is the kernel command-line args for enabling SELinux and force it to be in
	// enforcing mode.
	CmdlineSELinuxForceEnforcing = CmdlineSELinuxSettings + " " + CmdlineSELinuxEnforcingArg

	// SELinuxConfigFile is the file path of the SELinux config file.
	SELinuxConfigFile = "/etc/selinux/config"

	// SELinuxConfigEnforcing is the string value to set SELinux to enforcing in the /etc/selinux/config file.
	SELinuxConfigEnforcing = "enforcing"

	// SELinuxConfigPermissive is the string value to set SELinux to permissive in the /etc/selinux/config file.
	SELinuxConfigPermissive = "permissive"

	// SELinuxConfigDisabled is the string value to set SELinux to disabled in the /etc/selinux/config file.
	SELinuxConfigDisabled = "disabled"

	// GrubCfgFile is the filepath of the grub config file.
	GrubCfgFile = "/boot/grub2/grub.cfg"

	// GrubDefFile is the filepath of the config file used by grub-mkconfig.
	GrubDefFile = "/etc/default/grub"

	// CombinedBootPartitionBootPrefix is the grub.cfg boot prefix used when the boot partition is the same as the
	// rootfs partition.
	CombinedBootPartitionBootPrefix = "/boot"
)

const (
	overlay        = "overlay"
	rootMountPoint = "/"
	bootMountPoint = "/boot"

	// rpmDependenciesDirectory is the directory which contains RPM database. It is not required for images that do not contain RPM.
	rpmDependenciesDirectory = "/var/lib/rpm"

	// rpmManifestDirectory is the directory containing manifests of installed packages to support distroless vulnerability scanning tools.
	rpmManifestDirectory = "/var/lib/rpmmanifest"

	// /boot directory should be only accesible by root. The directories need the execute bit as well.
	bootDirectoryFileMode = 0400
	bootDirectoryDirMode  = 0700

	// Configuration files related to boot behavior. Users should be able to read these files, and root should have RW access.
	bootUsrConfigFileMode = 0644
)

// CreateMountPointPartitionMap creates a map between the mountpoint supplied in the config file and the device path
// of the partition
// - partDevPathMap is a map of partition IDs to partition device paths
// - partIDToFsTypeMap is a map of partition IDs to filesystem type
// - config is the SystemConfig from a config file
// Output
// - mountPointDevPathMap is a map of mountpoint to partition device path
// - mountPointToFsTypeMap is a map of mountpoint to filesystem type
// - mountPointToMountArgsMap is a map of mountpoint to mount arguments to be passed on a call to mount
// - diffDiskBuild is a flag that denotes whether this is a diffdisk build or not
func CreateMountPointPartitionMap(partDevPathMap, partIDToFsTypeMap map[string]string, partitionSettings []configuration.PartitionSetting) (mountPointDevPathMap, mountPointToFsTypeMap, mountPointToMountArgsMap map[string]string, diffDiskBuild bool) {
	mountPointDevPathMap = make(map[string]string)
	mountPointToFsTypeMap = make(map[string]string)
	mountPointToMountArgsMap = make(map[string]string)

	// Go through each PartitionSetting
	for _, partitionSetting := range partitionSettings {
		logger.Log.Tracef("%v[%v]", partitionSetting.ID, partitionSetting.MountPoint)
		partDevPath, ok := partDevPathMap[partitionSetting.ID]
		if ok {
			if partitionSetting.OverlayBaseImage == "" {
				mountPointDevPathMap[partitionSetting.MountPoint] = partDevPath
				mountPointToFsTypeMap[partitionSetting.MountPoint] = partIDToFsTypeMap[partitionSetting.ID]
				mountPointToMountArgsMap[partitionSetting.MountPoint] = partitionSetting.MountOptions
			} else {
				diffDiskBuild = true
			}
		}
		logger.Log.Tracef("%v", mountPointDevPathMap)
	}
	return
}

// ClearSystemdState clears the systemd state files that should be unique to each instance of the image. This is
// based on https://systemd.io/BUILDING_IMAGES/. Primarily, this function will ensure that /etc/machine-id is configured
// correctly, and that random seed and credential files are removed if they exist.
// - installChroot is the chroot to modify
// - enableSystemdFirstboot will set the machine-id file to "uninitialized" if true, and "" if false
func ClearSystemdState(installChroot *safechroot.Chroot, enableSystemdFirstboot bool) (err error) {
	const (
		machineIDFile         = "/etc/machine-id"
		machineIDFirstBootOn  = "uninitialized\n"
		machineIDFirstbootOff = ""
		machineIDFilePerms    = 0444
	)

	// These state files are very unlikely to be present, but we should be thorough and check for them.
	// See https://systemd.io/BUILDING_IMAGES/ for more information.
	var otherFilesToRemove = []string{
		"/var/lib/systemd/random-seed",
		"/boot/efi/loader/random-seed",
		"/var/lib/systemd/credential.secret",
	}

	// From https://www.freedesktop.org/software/systemd/man/latest/machine-id.html#Initialization:
	// For operating system images which are created once and used on multiple
	// machines, for example for containers or in the cloud, /etc/machine-id
	// should be either missing or an empty file in the generic file system
	// image (the difference between the two options is described under
	// "First Boot Semantics" below). An ID will be generated during boot and
	// saved to this file if possible. Having an empty file in place is useful
	// because it allows a temporary file to be bind-mounted over the real file,
	// in case the image is used read-only
	//
	// From https://www.freedesktop.org/software/systemd/man/latest/machine-id.html#First%20Boot%20Semantics:
	//     etc/machine-id is used to decide whether a boot is the first one. The rules are as follows:
	//     1. The kernel command argument systemd.condition-first-boot= may be used to override the autodetection logic,
	//         see kernel-command-line(7).
	//     2. Otherwise, if /etc/machine-id does not exist, this is a first boot. During early boot, systemd will write
	//         "uninitialized\n" to this file and overmount a temporary file which contains the actual machine ID. Later
	//         (after first-boot-complete.target has been reached), the real machine ID will be written to disk.
	//     3. If /etc/machine-id contains the string "uninitialized", a boot is also considered the first boot. The same
	//         mechanism as above applies.
	//     4. If /etc/machine-id exists and is empty, a boot is not considered the first boot. systemd will still
	//         bind-mount a file containing the actual machine-id over it and later try to commit it to disk (if /etc/ is
	//         writable).
	//     5. If /etc/machine-id already contains a valid machine-id, this is not a first boot.
	//     If according to the above rules a first boot is detected, units with ConditionFirstBoot=yes will be run and
	//     systemd will perform additional initialization steps, in particular presetting units.
	//
	// We will use option 4) by default since AZL has traditionally not used firstboot mechanisms. All configuration
	// that systemd-firstboot would set should have already been configured by the imager tool. It is important to
	// create an empty file so that read-only configurations will work as expected. If the user requests that firstboot
	// be enabled we will set it to "uninitalized" as per option 3).

	ReportAction("Configuring systemd state files for first boot")

	// The systemd package will create this file, but if its not installed, we need to create it.
	exists, err := file.PathExists(filepath.Join(installChroot.RootDir(), machineIDFile))
	if err != nil {
		err = fmt.Errorf("failed to check if machine-id exists:\n%w", err)
		return
	}
	if !exists {
		logger.Log.Debug("Creating empty machine-id file")
		err = file.Create(filepath.Join(installChroot.RootDir(), machineIDFile), machineIDFilePerms)
		if err != nil {
			err = fmt.Errorf("failed to create empty machine-id:\n%w", err)
			return err
		}
	}

	if enableSystemdFirstboot {
		ReportAction("Enabling systemd firstboot")
		err = file.Write(machineIDFirstBootOn, filepath.Join(installChroot.RootDir(), machineIDFile))
	} else {
		ReportAction("Disabling systemd firstboot")
		err = file.Write(machineIDFirstbootOff, filepath.Join(installChroot.RootDir(), machineIDFile))
	}
	if err != nil {
		err = fmt.Errorf("failed to write empty machine-id:\n%w", err)
		return err
	}

	// These files should not be present in the image, but per https://systemd.io/BUILDING_IMAGES/ we should
	// be thorough and double-check.
	for _, filePath := range otherFilesToRemove {
		fullPath := filepath.Join(installChroot.RootDir(), filePath)
		exists, err = file.PathExists(fullPath)
		if err != nil {
			err = fmt.Errorf("failed to check if systemd state file (%s) exists:\n%w", filePath, err)
			return err
		}

		// Do an explicit check for existence so we can log the file removal.
		if exists {
			ReportActionf("Removing systemd state file (%s)", filePath)
			err = file.RemoveFileIfExists(fullPath)
			if err != nil {
				err = fmt.Errorf("failed to remove systemd state file (%s):\n%w", filePath, err)
				return err
			}
		}
	}

	return
}

func UpdateFstabFile(fullFstabPath string, partitionSettings []configuration.PartitionSetting, mountList []string,
	mountPointMap, mountPointToFsTypeMap, mountPointToMountArgsMap, partIDToDevPathMap, partIDToFsTypeMap map[string]string,
	hidepidEnabled bool,
) (err error) {
	const (
		doPseudoFsMount = true
	)
	ReportAction("Configuring fstab")

	for _, mountPoint := range mountList {
		devicePath := mountPointMap[mountPoint]

		if mountPoint != "" && devicePath != NullDevice {
			partSetting := configuration.FindMountpointPartitionSetting(partitionSettings, mountPoint)
			if partSetting == nil {
				err = fmt.Errorf("unable to find PartitionSetting for '%s", mountPoint)
				return
			}
			err = addEntryToFstab(fullFstabPath, mountPoint, devicePath, mountPointToFsTypeMap[mountPoint],
				mountPointToMountArgsMap[mountPoint], partSetting.MountIdentifier, !doPseudoFsMount)
			if err != nil {
				return
			}
		}
	}

	if hidepidEnabled {
		err = addEntryToFstab(fullFstabPath, "/proc", "proc", "proc", "rw,nosuid,nodev,noexec,relatime,hidepid=2", configuration.MountIdentifierNone, doPseudoFsMount)
		if err != nil {
			return
		}
	}

	// Add swap entry if there is one
	for partID, fstype := range partIDToFsTypeMap {
		if fstype == "linux-swap" {
			swapPartitionPath, exists := partIDToDevPathMap[partID]
			if exists {
				err = addEntryToFstab(fullFstabPath, "none", swapPartitionPath, "swap", "", "", doPseudoFsMount)
				if err != nil {
					return
				}
			}
		}
	}

	return
}

func addEntryToFstab(fullFstabPath, mountPoint, devicePath, fsType, mountArgs string, identifierType configuration.MountIdentifier, doPseudoFsMount bool) (err error) {
	const (
		rootfsMountPoint = "/"
		defaultOptions   = "defaults"
		swapFsType       = "swap"
		swapOptions      = "sw"
		defaultDump      = "0"
		disablePass      = "0"
		rootPass         = "1"
		defaultPass      = "2"
	)

	var options string

	if mountArgs == "" {
		options = defaultOptions
	} else {
		options = mountArgs
	}

	if fsType == swapFsType {
		options = swapOptions
	}

	// Get the block device
	var device string
	if diskutils.IsEncryptedDevice(devicePath) || doPseudoFsMount {
		device = devicePath
	} else {
		device, err = FormatMountIdentifier(identifierType, devicePath)
		if err != nil {
			logger.Log.Warnf("Failed to get mount identifier for block device %v", devicePath)
			return err
		}
	}

	// Note: Rootfs should always have a pass number of 1. All other mountpoints are either 0 or 2
	pass := defaultPass
	if mountPoint == rootfsMountPoint {
		pass = rootPass
	} else if doPseudoFsMount {
		pass = disablePass
	}

	// Construct fstab entry and append to fstab file
	newEntry := fmt.Sprintf("%v %v %v %v %v %v\n", device, mountPoint, fsType, options, defaultDump, pass)
	err = file.Append(newEntry, fullFstabPath)
	if err != nil {
		logger.Log.Warnf("Failed to append to fstab file")
		return
	}
	return
}

func ConfigureDiskBootloaderWithRootMountIdType(bootType string, encryptionEnable bool,
	rootMountIdentifier configuration.MountIdentifier, kernelCommandLine configuration.KernelCommandLine,
	installChroot *safechroot.Chroot, diskDevPath string, mountPointMap map[string]string,
	encryptedRoot diskutils.EncryptedRootDevice, enableGrubMkconfig bool, includeLegacyGrubCfg bool,
) (err error) {
	// Add bootloader. Prefer a separate boot partition if one exists.
	bootDevice, isBootPartitionSeparate := mountPointMap[bootMountPoint]
	bootPrefix := ""
	if !isBootPartitionSeparate {
		bootDevice = mountPointMap[rootMountPoint]
		// If we do not have a separate boot partition we will need to add a prefix to all paths used in the configs.
		bootPrefix = CombinedBootPartitionBootPrefix
	}

	if mountPointMap[rootMountPoint] == NullDevice {
		// In case of overlay device being mounted at root, no need to change the bootloader.
		return
	}

	// Grub only accepts UUID, not PARTUUID or PARTLABEL
	bootUUID, err := GetUUID(bootDevice)
	if err != nil {
		err = fmt.Errorf("failed to get UUID: %s", err)
		return
	}

	err = InstallBootloader(installChroot, encryptionEnable, bootType, bootUUID, bootPrefix, diskDevPath)
	if err != nil {
		err = fmt.Errorf("failed to install bootloader: %s", err)
		return
	}

	// Add grub config to image
	var rootDevice string
	if encryptionEnable {
		// Encrypted devices don't currently support identifiers
		rootDevice = mountPointMap[rootMountPoint]
	} else {
		var partIdentifier string
		partIdentifier, err = FormatMountIdentifier(rootMountIdentifier, mountPointMap[rootMountPoint])
		if err != nil {
			err = fmt.Errorf("failed to get partIdentifier: %s", err)
			return
		}

		rootDevice = partIdentifier
	}

	// Grub will always use filesystem UUID, never PARTUUID or PARTLABEL
	err = InstallGrubDefaults(installChroot.RootDir(), rootDevice, bootUUID, bootPrefix, encryptedRoot,
		kernelCommandLine, isBootPartitionSeparate, includeLegacyGrubCfg)
	if err != nil {
		err = fmt.Errorf("failed to install main grub config file: %s", err)
		return
	}

	err = InstallGrubEnv(installChroot.RootDir())
	if err != nil {
		err = fmt.Errorf("failed to install grubenv file: %s", err)
		return
	}

	// Use grub mkconfig to replace the static template .cfg with a dynamically generated version if desired.
	if enableGrubMkconfig {
		err = CallGrubMkconfig(installChroot)
		if err != nil {
			err = fmt.Errorf("failed to generate grub.cfg via grub2-mkconfig: %s", err)
			return
		}
	}

	return
}

// InstallGrubEnv installs an empty grubenv f
func InstallGrubEnv(installRoot string) (err error) {
	const (
		assetGrubEnvFile = "assets/grub2/grubenv"
		grubEnvFile      = "boot/grub2/grubenv"
	)
	installGrubEnvFile := filepath.Join(installRoot, grubEnvFile)
	err = file.CopyResourceFile(resources.ResourcesFS, assetGrubEnvFile, installGrubEnvFile, bootDirectoryDirMode,
		bootDirectoryFileMode)
	if err != nil {
		logger.Log.Warnf("Failed to copy and change mode of grubenv: %v", err)
		return
	}

	return
}

// InstallGrubDefaults installs the main grub config to the rootfs partition
// - installRoot is the base install directory
// - rootDevice holds the root partition
// - bootUUID is the UUID for the boot partition
// - bootPrefix is the path to the /boot grub configs based on the mountpoints (i.e., if /boot is a separate partition from the rootfs partition, bootPrefix="").
// - encryptedRoot holds the encrypted root information if encrypted root is enabled
// - kernelCommandLine contains additional kernel parameters which may be optionally set
// - readOnlyRoot holds the dm-verity read-only root partition information if dm-verity is enabled.
// - isBootPartitionSeparate is a boolean value which is true if the /boot partition is separate from the root partition
// - includeLegacyCfg specifies if the legacy grub.cfg from Azure Linux should also be added.
// Note: this boot partition could be different than the boot partition specified in the bootloader.
// This boot partition specifically indicates where to find the kernel, config files, and initrd
func InstallGrubDefaults(installRoot, rootDevice, bootUUID, bootPrefix string,
	encryptedRoot diskutils.EncryptedRootDevice, kernelCommandLine configuration.KernelCommandLine,
	isBootPartitionSeparate bool, includeLegacyCfg bool,
) (err error) {
	// Copy the bootloader's /etc/default/grub and set the file permission
	err = installGrubTemplateFile(resources.AssetsGrubDefFile, GrubDefFile, installRoot, rootDevice, bootUUID,
		bootPrefix, encryptedRoot, kernelCommandLine, isBootPartitionSeparate)
	if err != nil {
		logger.Log.Warnf("Failed to install (%s): %v", GrubDefFile, err)
		return
	}

	if includeLegacyCfg {
		// Add the legacy /boot/grub2/grub.cfg file, which was used in Azure Linux 2.0.
		err = installGrubTemplateFile(resources.AssetsGrubCfgFile, GrubCfgFile, installRoot, rootDevice, bootUUID,
			bootPrefix, encryptedRoot, kernelCommandLine, isBootPartitionSeparate)
		if err != nil {
			logger.Log.Warnf("Failed to install (%s): %v", GrubCfgFile, err)
			return
		}
	}

	return
}

func installGrubTemplateFile(assetFile, targetFile, installRoot, rootDevice, bootUUID, bootPrefix string,
	encryptedRoot diskutils.EncryptedRootDevice, kernelCommandLine configuration.KernelCommandLine,
	isBootPartitionSeparate bool,
) (err error) {
	installGrubDefFile := filepath.Join(installRoot, targetFile)

	err = file.CopyResourceFile(resources.ResourcesFS, assetFile, installGrubDefFile, bootDirectoryDirMode,
		bootUsrConfigFileMode)
	if err != nil {
		return
	}

	// Add in bootUUID
	err = setGrubCfgBootUUID(bootUUID, installGrubDefFile)
	if err != nil {
		logger.Log.Warnf("Failed to set bootUUID in %s: %v", installGrubDefFile, err)
		return
	}

	// Add in bootPrefix
	err = setGrubCfgBootPrefix(bootPrefix, installGrubDefFile)
	if err != nil {
		logger.Log.Warnf("Failed to set bootPrefix in %s: %v", installGrubDefFile, err)
		return
	}

	// Add in rootDevice
	err = setGrubCfgRootDevice(rootDevice, installGrubDefFile, encryptedRoot.LuksUUID)
	if err != nil {
		logger.Log.Warnf("Failed to set rootDevice in %s: %v", installGrubDefFile, err)
		return
	}

	// Add in rootLuksUUID
	err = setGrubCfgLuksUUID(installGrubDefFile, encryptedRoot.LuksUUID)
	if err != nil {
		logger.Log.Warnf("Failed to set luksUUID in %s: %v", installGrubDefFile, err)
		return
	}

	// Add in logical volumes to active
	err = setGrubCfgLVM(installGrubDefFile, encryptedRoot.LuksUUID)
	if err != nil {
		logger.Log.Warnf("Failed to set lvm.lv in %s: %v", installGrubDefFile, err)
		return
	}

	// Configure IMA policy
	err = setGrubCfgIMA(installGrubDefFile, kernelCommandLine)
	if err != nil {
		logger.Log.Warnf("Failed to set ima_policy in in %s: %v", installGrubDefFile, err)
		return
	}

	err = setGrubCfgSELinux(installGrubDefFile, kernelCommandLine)
	if err != nil {
		logger.Log.Warnf("Failed to set SELinux in %s: %v", installGrubDefFile, err)
		return
	}

	// Configure FIPS
	err = setGrubCfgFIPS(isBootPartitionSeparate, bootUUID, installGrubDefFile, kernelCommandLine)
	if err != nil {
		logger.Log.Warnf("Failed to set FIPS in %s: %v", installGrubDefFile, err)
		return
	}

	err = setGrubCfgCGroup(installGrubDefFile, kernelCommandLine)
	if err != nil {
		logger.Log.Warnf("Failed to set CGroup configuration in %s: %v", installGrubDefFile, err)
		return
	}

	// Append any additional command line parameters
	err = setGrubCfgAdditionalCmdLine(installGrubDefFile, kernelCommandLine)
	if err != nil {
		logger.Log.Warnf("Failed to append extra command line parameters in %s: %v", installGrubDefFile, err)
		return
	}

	return
}

func CallGrubMkconfig(installChroot safechroot.ChrootInterface) (err error) {
	squashErrors := true

	ReportActionf("Running grub2-mkconfig...")
	err = installChroot.UnsafeRun(func() error {
		return shell.ExecuteLive(squashErrors, "grub2-mkconfig", "-o", GrubCfgFile)
	})

	return
}

// chage works in the same way as invoking "chage -M passwordExpirationInDays username"
// i.e. it sets the maximum password expiration date.
func Chage(installChroot safechroot.ChrootInterface, passwordExpirationInDays int64, username string) (err error) {
	var (
		shadow            []string
		usernameWithColon = fmt.Sprintf("%s:", username)
	)

	installChrootShadowFile := filepath.Join(installChroot.RootDir(), userutils.ShadowFile)

	shadow, err = file.ReadLines(installChrootShadowFile)
	if err != nil {
		return
	}

	for n, entry := range shadow {
		done := false
		// Entries in shadow are separated by colon and start with a username
		// Finding one that starts like that means we've found our entry
		if strings.HasPrefix(entry, usernameWithColon) {
			// Each line in shadow contains 9 fields separated by colon ("") in the following order:
			// login name, encrypted password, date of last password change,
			// minimum password age, maximum password age, password warning period,
			// password inactivity period, account expiration date, reserved field for future use
			const (
				passwordNeverExpiresValue = -1
				loginNameField            = 0
				encryptedPasswordField    = 1
				passwordChangedField      = 2
				minPasswordAgeField       = 3
				maxPasswordAgeField       = 4
				warnPeriodField           = 5
				inactivityPeriodField     = 6
				expirationField           = 7
				reservedField             = 8
				totalFieldsCount          = 9
			)

			fields := strings.Split(entry, ":")
			// Any value other than totalFieldsCount indicates error in parsing
			if len(fields) != totalFieldsCount {
				return fmt.Errorf("invalid shadow entry (%v) for user (%s): %d fields expected, but %d found", fields, username, totalFieldsCount, len(fields))
			}

			if passwordExpirationInDays == passwordNeverExpiresValue {
				// If passwordExpirationInDays is equal to -1, it means that password never expires.
				// This is expressed by leaving account expiration date field (and fields after it) empty.
				for _, fieldToChange := range []int{maxPasswordAgeField, warnPeriodField, inactivityPeriodField, expirationField, reservedField} {
					fields[fieldToChange] = ""
				}
				// Each user appears only once, since we found one, we are finished; save the changes and exit.
				done = true
			} else if passwordExpirationInDays < passwordNeverExpiresValue {
				// Values smaller than -1 make no sense
				return fmt.Errorf("invalid value for maximum user's (%s) password expiration: %d; should be greater than %d", username, passwordExpirationInDays, passwordNeverExpiresValue)
			} else {
				// If passwordExpirationInDays has any other value, it's the maximum expiration date: set it accordingly
				// To do so, we need to ensure that passwordChangedField holds a valid value and then sum it with passwordExpirationInDays.
				var (
					passwordAge     int64
					passwordChanged = fields[passwordChangedField]
				)

				if passwordChanged == "" {
					// Set to the number of days since epoch
					fields[passwordChangedField] = fmt.Sprintf("%d", DaysSinceUnixEpoch())
				}
				passwordAge, err = strconv.ParseInt(fields[passwordChangedField], 10, 64)
				if err != nil {
					return
				}
				fields[expirationField] = fmt.Sprintf("%d", passwordAge+passwordExpirationInDays)

				// Each user appears only once, since we found one, we are finished; save the changes and exit.
				done = true
			}
			if done {
				// Create and save new shadow file including potential changes from above.
				shadow[n] = strings.Join(fields, ":")
				err = file.Write(strings.Join(shadow, "\n"), installChrootShadowFile)
				return
			}
		}
	}

	return fmt.Errorf(`user "%s" not found when trying to change the password expiration date`, username)
}

func DaysSinceUnixEpoch() int64 {
	return int64(time.Since(time.Unix(0, 0)).Hours() / 24)
}

func ConfigureUserPrimaryGroupMembership(installChroot safechroot.ChrootInterface, username string, primaryGroup string,
) (err error) {
	if primaryGroup != "" {
		err = installChroot.UnsafeRun(func() error {
			return shell.ExecuteLiveWithErr(1, "usermod", "-g", primaryGroup, username)
		})

		if err != nil {
			return fmt.Errorf("failed to set user's (%s) primary group (%s):\n%w", username, primaryGroup, err)
		}
	}

	return
}

func ConfigureUserSecondaryGroupMembership(installChroot safechroot.ChrootInterface, username string, secondaryGroups []string,
) (err error) {
	if len(secondaryGroups) != 0 {
		allGroups := strings.Join(secondaryGroups, ",")
		err = installChroot.UnsafeRun(func() error {
			return shell.ExecuteLiveWithErr(1, "usermod", "-a", "-G", allGroups, username)
		})
		if err != nil {
			return fmt.Errorf("failed to set user's (%s) secondary groups:\n%w", username, err)
		}
	}

	return
}

func ConfigureUserStartupCommand(installChroot safechroot.ChrootInterface, username string, startupCommand string) (err error) {
	const (
		sedDelimiter = "|"
	)

	if startupCommand == "" {
		return
	}

	logger.Log.Debugf("Updating user '%s' startup command to '%s'.", username, startupCommand)

	findPattern := fmt.Sprintf(`^\(%s.*\):[^:]*$`, username)
	replacePattern := fmt.Sprintf(`\1:%s`, startupCommand)
	filePath := filepath.Join(installChroot.RootDir(), userutils.PasswdFile)
	err = sed(findPattern, replacePattern, sedDelimiter, filePath)
	if err != nil {
		err = fmt.Errorf("failed to update user's (%s) startup command (%s):\n%w", username, startupCommand, err)
		return
	}

	return
}

func SELinuxUpdateConfig(selinuxMode configuration.SELinux, installChroot safechroot.ChrootInterface) (err error) {
	const (
		selinuxPattern = "^SELINUX=.*"
	)
	var mode string

	switch selinuxMode {
	case configuration.SELinuxEnforcing, configuration.SELinuxForceEnforcing:
		mode = SELinuxConfigEnforcing
	case configuration.SELinuxPermissive:
		mode = SELinuxConfigPermissive
	case configuration.SELinuxOff:
		mode = SELinuxConfigDisabled
	}

	selinuxConfigPath := filepath.Join(installChroot.RootDir(), SELinuxConfigFile)
	selinuxProperty := fmt.Sprintf("SELINUX=%s", mode)
	err = sed(selinuxPattern, selinuxProperty, "`", selinuxConfigPath)
	return
}

func SELinuxRelabelFiles(installChroot safechroot.ChrootInterface, mountPointToFsTypeMap map[string]string, isRootFS bool,
) (err error) {
	const (
		fileContextBasePath = "etc/selinux/%s/contexts/files/file_contexts"
	)
	var listOfMountsToLabel []string

	if isRootFS {
		listOfMountsToLabel = append(listOfMountsToLabel, "/")
	} else {
		// Search through all our mount points for supported filesystem types
		// Note for the future: SELinux can support any of {btrfs, encfs, ext2-4, f2fs, jffs2, jfs, ubifs, xfs, zfs}, but the build system currently
		//     only supports the below cases:
		for mount, fsType := range mountPointToFsTypeMap {
			switch fsType {
			case "ext2", "ext3", "ext4", "xfs", "btrfs":
				listOfMountsToLabel = append(listOfMountsToLabel, mount)
			case "fat32", "fat16", "vfat":
				logger.Log.Debugf("SELinux will not label mount at (%s) of type (%s), skipping", mount, fsType)
			default:
				err = fmt.Errorf("unknown fsType (%s) for mount (%s), cannot configure SELinux", fsType, mount)
				return
			}
		}
	}

	// Find the type of policy we want to label with
	selinuxConfigPath := filepath.Join(installChroot.RootDir(), SELinuxConfigFile)
	stdout, stderr, err := shell.Execute("sed", "-n", "s/^SELINUXTYPE=\\(.*\\)$/\\1/p", selinuxConfigPath)
	if err != nil {
		err = fmt.Errorf("failed to find an SELINUXTYPE in (%s):\n%w\n%v", selinuxConfigPath, err, stderr)
		return
	}
	selinuxType := strings.TrimSpace(stdout)
	fileContextPath := fmt.Sprintf(fileContextBasePath, selinuxType)

	targetRootPath := "/mnt/_bindmountroot"
	targetRootFullPath := filepath.Join(installChroot.RootDir(), targetRootPath)

	for _, mountToLabel := range listOfMountsToLabel {
		logger.Log.Debugf("Running setfiles to apply SELinux labels on mount points: %v", mountToLabel)

		// The chroot environment has a bunch of special filesystems (e.g. /dev, /proc, etc.) mounted within the OS
		// image. In addition, an image may have placed system directories on separate partitions, and these partitions
		// will also be mounted within the OS image. These mounts hide the underlying directory that is used as a mount
		// point, which prevents that directory from receiving an SELinux label from the setfiles command. A well known
		// way to get an unobstructed view of a filesystem, free from other mount-points, is to create a bind-mount for
		// that filesystem. Therefore, bind mounts are used to ensure that all directories receive an SELinux label.
		sourceFullPath := filepath.Join(installChroot.RootDir(), mountToLabel)
		targetPath := filepath.Join(targetRootPath, mountToLabel)
		targetFullPath := filepath.Join(installChroot.RootDir(), targetPath)

		bindMount, err := safemount.NewMount(sourceFullPath, targetFullPath, "", unix.MS_BIND, "", true)
		if err != nil {
			return fmt.Errorf("failed to bind mount (%s) while SELinux labeling:\n%w", mountToLabel, err)
		}
		defer bindMount.Close()

		err = installChroot.UnsafeRun(func() error {
			// We only want to print basic info, filter out the real output unless at trace level (Execute call handles that)
			files := 0
			onStdout := func(line string) {
				files++
				if (files % 1000) == 0 {
					logger.Log.Debugf("SELinux: labelled %d files", files)
				}
			}
			err := shell.NewExecBuilder("setfiles", "-m", "-v", "-r", targetRootPath, fileContextPath, targetPath).
				StdoutCallback(onStdout).
				LogLevel(logrus.TraceLevel, logrus.WarnLevel).
				ErrorStderrLines(1).
				Execute()
			if err != nil {
				return fmt.Errorf("setfiles failed:\n%w", err)
			}
			logger.Log.Debugf("SELinux: labelled %d files", files)
			return err
		})
		if err != nil {
			return err
		}

		err = bindMount.CleanClose()
		if err != nil {
			return err
		}

		// Cleanup the temporary directory.
		// Note: This is intentionally done within the for loop to ensure the directory is always empty for the next
		// mount. For example, if a parent directory mount is processed after a nested child directory mount.
		err = os.RemoveAll(targetRootFullPath)
		if err != nil {
			return fmt.Errorf("failed to remove temporary bind mount directory:\n%w", err)
		}
	}

	return
}

func sed(find, replace, delimiter, file string) (err error) {
	const squashErrors = false

	replacement := fmt.Sprintf("s%s%s%s%s%s", delimiter, find, delimiter, replace, delimiter)
	return shell.ExecuteLive(squashErrors, "sed", "-i", replacement, file)
}

// InstallBootloader installs the proper bootloader for this type of image
// - installChroot is a pointer to the install Chroot object
// - bootType indicates the type of boot loader to add.
// - bootUUID is the UUID of the boot partition
// Note: this boot partition could be different than the boot partition specified in the main grub config.
// This boot partition specifically indicates where to find the main grub cfg
func InstallBootloader(installChroot *safechroot.Chroot, encryptEnabled bool, bootType, bootUUID, bootPrefix,
	bootDevPath string,
) (err error) {
	const (
		efiMountPoint  = "/boot/efi"
		efiBootType    = "efi"
		legacyBootType = "legacy"
		noneBootType   = "none"
	)

	ReportAction("Configuring bootloader")

	switch bootType {
	case legacyBootType:
		err = installLegacyBootloader(installChroot, bootDevPath, encryptEnabled)
		if err != nil {
			return
		}
	case efiBootType:
		efiPath := filepath.Join(installChroot.RootDir(), efiMountPoint)
		err = installEfiBootloader(encryptEnabled, efiPath, bootUUID, bootPrefix)
		if err != nil {
			return
		}
	case noneBootType:
		// Nothing to do here
	default:
		err = fmt.Errorf("unknown boot type: %v", bootType)
		return
	}
	return
}

// Note: We assume that the /boot directory is present. Whether it is backed by an explicit "boot" partition or present
// as part of a general "root" partition is assumed to have been done already.
func installLegacyBootloader(installChroot *safechroot.Chroot, bootDevPath string, encryptEnabled bool) (err error) {
	const (
		squashErrors     = false
		bootDir          = "/boot"
		bootDirArg       = "--boot-directory"
		grub2BootDir     = "/boot/grub2"
		grub2InstallName = "grub2-install"
		grubInstallName  = "grub-install"
	)

	// Add grub cryptodisk settings
	if encryptEnabled {
		err = enableCryptoDisk()
		if err != nil {
			return
		}
	}
	installBootDir := filepath.Join(installChroot.RootDir(), bootDir)
	grub2InstallBootDirArg := fmt.Sprintf("%s=%s", bootDirArg, installBootDir)

	installName := grub2InstallName
	grub2InstallExists, err := file.CommandExists(grub2InstallName)
	if err != nil {
		return
	}

	if !grub2InstallExists {
		grubInstallExists, err := file.CommandExists(grubInstallName)
		if err != nil {
			return err
		}

		if !grubInstallExists {
			return fmt.Errorf("neither 'grub2-install' command nor 'grub-install' command found")
		}

		installName = grubInstallName
	}

	err = shell.ExecuteLive(squashErrors, installName, "--target=i386-pc", grub2InstallBootDirArg, bootDevPath)
	if err != nil {
		return
	}

	installGrub2BootDir := filepath.Join(installChroot.RootDir(), grub2BootDir)
	err = shell.ExecuteLive(squashErrors, "chmod", "-R", "go-rwx", installGrub2BootDir)
	return
}

// GetUUID queries the UUID of the given partition
// - device is the device path of the desired partition
func GetUUID(device string) (stdout string, err error) {
	stdout, _, err = shell.Execute("blkid", device, "-s", "UUID", "-o", "value")
	if err != nil {
		return
	}
	logger.Log.Trace(stdout)
	stdout = strings.TrimSpace(stdout)
	return
}

// GetPartUUID queries the PARTUUID of the given partition
// - device is the device path of the desired partition
func GetPartUUID(device string) (stdout string, err error) {
	stdout, _, err = shell.Execute("blkid", device, "-s", "PARTUUID", "-o", "value")
	if err != nil {
		return
	}
	logger.Log.Trace(stdout)
	stdout = strings.TrimSpace(stdout)
	return
}

// GetPartLabel queries the PARTLABEL of the given partition
// - device is the device path of the desired partition
func GetPartLabel(device string) (stdout string, err error) {
	stdout, _, err = shell.Execute("blkid", device, "-s", "PARTLABEL", "-o", "value")
	if err != nil {
		return
	}
	logger.Log.Trace(stdout)
	stdout = strings.TrimSpace(stdout)
	return
}

// FormatMountIdentifier finds the requested identifier type for the given device, and formats it for use
//
//	ie "UUID=12345678-abcd..."
func FormatMountIdentifier(identifier configuration.MountIdentifier, device string) (identifierString string, err error) {
	var id string
	switch identifier {
	case configuration.MountIdentifierUuid:
		id, err = GetUUID(device)
		if err != nil {
			return
		}
		identifierString = fmt.Sprintf("UUID=%s", id)
	case configuration.MountIdentifierPartUuid:
		id, err = GetPartUUID(device)
		if err != nil {
			return
		}
		identifierString = fmt.Sprintf("PARTUUID=%s", id)
	case configuration.MountIdentifierPartLabel:
		id, err = GetPartLabel(device)
		if err != nil {
			return
		}
		identifierString = fmt.Sprintf("PARTLABEL=%s", id)
	case configuration.MountIdentifierNone:
		err = fmt.Errorf("must select a mount identifier for device (%s)", device)
	default:
		err = fmt.Errorf("unknown mount identifier: (%v)", identifier)
	}
	return
}

// enableCryptoDisk enables Grub to boot from an encrypted disk
// - installChroot is the installation chroot
func enableCryptoDisk() (err error) {
	const (
		grubCryptoDisk     = "GRUB_ENABLE_CRYPTODISK=y\n"
		grubPreloadModules = `GRUB_PRELOAD_MODULES="lvm"`
	)

	err = file.Append(grubCryptoDisk, GrubDefFile)
	if err != nil {
		logger.Log.Warnf("Failed to add grub cryptodisk: %v", err)
		return
	}
	err = file.Append(grubPreloadModules, GrubDefFile)
	if err != nil {
		logger.Log.Warnf("Failed to add grub preload modules: %v", err)
		return
	}
	return
}

// installEfi copies the efi binaries and grub configuration to the appropriate
// installRoot/boot/efi folder
// It is expected that shim (bootx64.efi) and grub2 (grub2.efi) are installed
// into the EFI directory via the package list installation mechanism.
func installEfiBootloader(encryptEnabled bool, installRoot, bootUUID, bootPrefix string) (err error) {
	const (
		defaultCfgFilename = "grub.cfg"
		grubAssetDir       = "assets/efi/grub"
		grubFinalDir       = "boot/grub2"
	)

	// Copy the bootloader's grub.cfg
	grubAssetPath := filepath.Join(grubAssetDir, defaultCfgFilename)
	grubFinalPath := filepath.Join(installRoot, grubFinalDir, defaultCfgFilename)
	err = file.CopyResourceFile(resources.ResourcesFS, grubAssetPath, grubFinalPath, bootDirectoryDirMode,
		bootDirectoryFileMode)
	if err != nil {
		logger.Log.Warnf("Failed to copy grub.cfg: %v", err)
		return
	}

	// Add in bootUUID
	err = setGrubCfgBootUUID(bootUUID, grubFinalPath)
	if err != nil {
		logger.Log.Warnf("Failed to set bootUUID in grub.cfg: %v", err)
		return
	}

	// Set the boot prefix path
	prefixPath := filepath.Join("/", bootPrefix, "grub2")
	err = setGrubCfgPrefixPath(prefixPath, grubFinalPath)
	if err != nil {
		logger.Log.Warnf("Failed to set prefixPath in grub.cfg: %v", err)
		return
	}

	// Add in encrypted volume mount command if needed
	err = setGrubCfgEncryptedVolume(grubFinalPath, encryptEnabled)
	if err != nil {
		logger.Log.Warnf("Failed to set encrypted volume in grub.cfg: %v", err)
		return
	}

	return
}

func setGrubCfgAdditionalCmdLine(grubPath string, kernelCommandline configuration.KernelCommandLine) (err error) {
	const (
		extraPattern = "{{.ExtraCommandLine}}"
	)

	logger.Log.Debugf("Adding ExtraCommandLine('%s') to '%s'", kernelCommandline.ExtraCommandLine, grubPath)
	err = sed(extraPattern, kernelCommandline.ExtraCommandLine, kernelCommandline.GetSedDelimeter(), grubPath)
	if err != nil {
		logger.Log.Warnf("Failed to append extra paramters to grub.cfg: %v", err)
	}

	return
}

func setGrubCfgIMA(grubPath string, kernelCommandline configuration.KernelCommandLine) (err error) {
	const (
		imaPrefix  = "ima_policy="
		imaPattern = "{{.IMAPolicy}}"
	)

	var ima string

	for _, policy := range kernelCommandline.ImaPolicy {
		ima += fmt.Sprintf("%v%v ", imaPrefix, policy)
	}

	logger.Log.Debugf("Adding ImaPolicy('%s') to '%s'", ima, grubPath)
	err = sed(imaPattern, ima, kernelCommandline.GetSedDelimeter(), grubPath)
	if err != nil {
		logger.Log.Warnf("Failed to set grub.cfg's IMA setting: %v", err)
	}

	return
}

func setGrubCfgSELinux(grubPath string, kernelCommandline configuration.KernelCommandLine) (err error) {
	const (
		selinuxPattern = "{{.SELinux}}"
	)
	var selinux string

	switch kernelCommandline.SELinux {
	case configuration.SELinuxForceEnforcing:
		selinux = CmdlineSELinuxForceEnforcing
	case configuration.SELinuxPermissive, configuration.SELinuxEnforcing:
		selinux = CmdlineSELinuxSettings
	case configuration.SELinuxOff:
		selinux = CmdlineSELinuxDisabledArg
	}

	logger.Log.Debugf("Adding SELinuxConfiguration('%s') to '%s'", selinux, grubPath)
	err = sed(selinuxPattern, selinux, kernelCommandline.GetSedDelimeter(), grubPath)
	if err != nil {
		logger.Log.Warnf("Failed to set grub.cfg's SELinux setting: %v", err)
	}

	return
}

func setGrubCfgFIPS(isBootPartitionSeparate bool, bootUUID, grubPath string, kernelCommandline configuration.KernelCommandLine) (err error) {
	const (
		enableFIPSPattern = "{{.FIPS}}"
		enableFIPS        = "fips=1"
		bootPrefix        = "boot="
		uuidPrefix        = "UUID="
	)

	// If EnableFIPS is set, always add "fips=1" to the kernel cmdline.
	// If /boot is a dedicated partition from the root partition, add "boot=UUID=<bootUUID value>" as well to the kernel cmdline in grub.cfg.
	// This second step is required for fips boot-time self tests to find the kernel's .hmac file in the /boot partition.
	fipsKernelArgument := ""
	if kernelCommandline.EnableFIPS {
		fipsKernelArgument = fmt.Sprintf("%s", enableFIPS)
		if isBootPartitionSeparate {
			fipsKernelArgument = fmt.Sprintf("%s %s%s%s", fipsKernelArgument, bootPrefix, uuidPrefix, bootUUID)
		}
	}

	logger.Log.Debugf("Adding EnableFIPS('%s') to '%s'", fipsKernelArgument, grubPath)
	err = sed(enableFIPSPattern, fipsKernelArgument, kernelCommandline.GetSedDelimeter(), grubPath)
	if err != nil {
		logger.Log.Warnf("Failed to set grub.cfg's EnableFIPS setting: %v", err)
	}
	return
}

func setGrubCfgCGroup(grubPath string, kernelCommandline configuration.KernelCommandLine) (err error) {
	const (
		cgroupPattern     = "{{.CGroup}}"
		cgroupv1FlagValue = "systemd.unified_cgroup_hierarchy=0"
		cgroupv2FlagValue = "systemd.unified_cgroup_hierarchy=1"
	)
	var cgroup string

	switch kernelCommandline.CGroup {
	case configuration.CGroupV2:
		cgroup = fmt.Sprintf("%s", cgroupv2FlagValue)
	case configuration.CGroupV1:
		cgroup = fmt.Sprintf("%s", cgroupv1FlagValue)
	case configuration.CGroupDefault:
		cgroup = ""
	}

	logger.Log.Debugf("Adding CGroupConfiguration('%s') to '%s'", cgroup, grubPath)
	err = sed(cgroupPattern, cgroup, kernelCommandline.GetSedDelimeter(), grubPath)
	if err != nil {
		logger.Log.Warnf("Failed to set grub.cfg's CGroup setting: %v", err)
	}

	return
}

func setGrubCfgLVM(grubPath, luksUUID string) (err error) {
	const (
		lvmPrefix  = "rd.lvm.lv="
		lvmPattern = "{{.LVM}}"
	)
	var cmdline configuration.KernelCommandLine

	var lvm string
	if luksUUID != "" {
		lvm = fmt.Sprintf("%v%v", lvmPrefix, diskutils.GetEncryptedRootVolPath())
	}

	logger.Log.Debugf("Adding lvm('%s') to '%s'", lvm, grubPath)
	err = sed(lvmPattern, lvm, cmdline.GetSedDelimeter(), grubPath)
	if err != nil {
		logger.Log.Warnf("Failed to set grub.cfg's LVM setting: %v", err)
	}

	return
}

func setGrubCfgLuksUUID(grubPath, uuid string) (err error) {
	const (
		luksUUIDPrefix  = "luks.uuid="
		luksUUIDPattern = "{{.LuksUUID}}"
	)
	var (
		cmdline  configuration.KernelCommandLine
		luksUUID string
	)
	if uuid != "" {
		luksUUID = fmt.Sprintf("%v%v", luksUUIDPrefix, uuid)
	}

	logger.Log.Debugf("Adding luks('%s') to '%s'", luksUUID, grubPath)
	err = sed(luksUUIDPattern, luksUUID, cmdline.GetSedDelimeter(), grubPath)
	if err != nil {
		logger.Log.Warnf("Failed to set grub.cfg's luksUUID: %v", err)
		return
	}

	return
}

func setGrubCfgBootUUID(bootUUID, grubPath string) (err error) {
	const (
		bootUUIDPattern = "{{.BootUUID}}"
	)
	var cmdline configuration.KernelCommandLine

	logger.Log.Debugf("Adding UUID('%s') to '%s'", bootUUID, grubPath)
	err = sed(bootUUIDPattern, bootUUID, cmdline.GetSedDelimeter(), grubPath)
	if err != nil {
		logger.Log.Warnf("Failed to set grub.cfg's bootUUID: %v", err)
		return
	}
	return
}

func setGrubCfgPrefixPath(prefixPath string, grubPath string) (err error) {
	const (
		prefixPathPattern = "{{.PrefixPath}}"
	)
	var cmdline configuration.KernelCommandLine

	err = sed(prefixPathPattern, prefixPath, cmdline.GetSedDelimeter(), grubPath)
	if err != nil {
		logger.Log.Warnf("Failed to set grub.cfg's prefixPath: %v", err)
		return
	}
	return
}

func setGrubCfgBootPrefix(bootPrefix, grubPath string) (err error) {
	const (
		bootPrefixPattern = "{{.BootPrefix}}"
	)
	var cmdline configuration.KernelCommandLine

	logger.Log.Debugf("Adding BootPrefix('%s') to '%s'", bootPrefix, grubPath)
	err = sed(bootPrefixPattern, bootPrefix, cmdline.GetSedDelimeter(), grubPath)
	if err != nil {
		logger.Log.Warnf("Failed to set grub.cfg's bootPrefix: %v", err)
		return
	}
	return
}

func setGrubCfgEncryptedVolume(grubPath string, enableEncryptedVolume bool) (err error) {
	const (
		encryptedVolPattern = "{{.CryptoMountCommand}}"
		cryptoMountCommand  = "cryptomount -a"
	)
	var (
		cmdline         configuration.KernelCommandLine
		encryptedVolArg = ""
	)

	if enableEncryptedVolume {
		encryptedVolArg = cryptoMountCommand
	}

	logger.Log.Debugf("Adding CryptoMountCommand('%s') to '%s'", encryptedVolArg, grubPath)
	err = sed(encryptedVolPattern, encryptedVolArg, cmdline.GetSedDelimeter(), grubPath)
	if err != nil {
		logger.Log.Warnf("Failed to grub.cfg's encryptedVolume: %v", err)
		return
	}
	return
}

func setGrubCfgRootDevice(rootDevice, grubPath, luksUUID string) (err error) {
	const (
		rootDevicePattern = "{{.RootPartition}}"
	)
	var cmdline configuration.KernelCommandLine

	if luksUUID != "" {
		rootDevice = diskutils.GetEncryptedRootVolMapping()
	}

	logger.Log.Debugf("Adding RootDevice('%s') to '%s'", rootDevice, grubPath)
	err = sed(rootDevicePattern, rootDevice, cmdline.GetSedDelimeter(), grubPath)
	if err != nil {
		logger.Log.Warnf("Failed to set grub.cfg's rootDevice: %v", err)
		return
	}
	return
}
