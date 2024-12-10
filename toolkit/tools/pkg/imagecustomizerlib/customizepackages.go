// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/sirupsen/logrus"
)

const (
	tdnfInstallPrefix = "Installing/Updating: "
	tdnfRemovePrefix  = "Removing: "
)

var (
	tdnfTransactionError = regexp.MustCompile(`^Found \d+ problems$`)
)

type PackageVersionInformation struct {
	PackageVersionComponents []uint64 `yaml:"PackageVersionComponents"`
	PackageRelease           uint32   `yaml:"PackageRelease"`
	DistroName               string   `yaml:"DistroName"`
	DistroVersion            uint32   `yaml:"DistroVersion"`
}

func (pi *PackageVersionInformation) getVersionString() (version string, err error) {
	if len(pi.PackageVersionComponents) == 0 {
		return "", fmt.Errorf("no version defined")
	}

	for i, versionComponent := range pi.PackageVersionComponents {
		if i != 0 {
			version += "."
		}
		version += strconv.FormatUint(versionComponent, 10)
	}
	return version, nil
}

func addRemoveAndUpdatePackages(buildDir string, baseConfigPath string, config *imagecustomizerapi.OS,
	imageChroot *safechroot.Chroot, rpmsSources []string, useBaseImageRpmRepos bool,
) error {
	var err error

	// Note: The 'validatePackageLists' function read the PackageLists files and merged them into the inline package lists.
	needRpmsSources := len(config.Packages.Install) > 0 || len(config.Packages.Update) > 0 ||
		config.Packages.UpdateExistingPackages

	var mounts *rpmSourcesMounts
	if needRpmsSources {
		// Mount RPM sources.
		mounts, err = mountRpmSources(buildDir, imageChroot, rpmsSources, useBaseImageRpmRepos)
		if err != nil {
			return err
		}
		defer mounts.close()

		// Refresh metadata.
		err = refreshTdnfMetadata(imageChroot)
		if err != nil {
			return err
		}
	}

	err = removePackages(config.Packages.Remove, imageChroot)
	if err != nil {
		return err
	}

	if config.Packages.UpdateExistingPackages {
		err = updateAllPackages(imageChroot)
		if err != nil {
			return err
		}
	}

	logger.Log.Infof("Installing packages: %v", config.Packages.Install)
	err = installOrUpdatePackages("install", config.Packages.Install, imageChroot)
	if err != nil {
		return err
	}

	logger.Log.Infof("Updating packages: %v", config.Packages.Update)
	err = installOrUpdatePackages("update", config.Packages.Update, imageChroot)
	if err != nil {
		return err
	}

	// Unmount RPM sources.
	if mounts != nil {
		err = mounts.close()
		if err != nil {
			return err
		}
	}

	if needRpmsSources {
		err = cleanTdnfCache(imageChroot)
		if err != nil {
			return err
		}
	}

	return nil
}

func refreshTdnfMetadata(imageChroot *safechroot.Chroot) error {
	tdnfArgs := []string{
		"-v", "check-update", "--refresh", "--nogpgcheck", "--assumeyes",
		"--setopt", fmt.Sprintf("reposdir=%s", rpmsMountParentDirInChroot),
	}

	err := imageChroot.UnsafeRun(func() error {
		return shell.NewExecBuilder("tdnf", tdnfArgs...).
			LogLevel(logrus.DebugLevel, logrus.DebugLevel).
			ErrorStderrLines(1).
			Execute()
	})
	if err != nil {
		return fmt.Errorf("failed to refresh tdnf repo metadata:\n%w", err)
	}
	return nil
}

func collectPackagesList(baseConfigPath string, packageLists []string, packages []string) ([]string, error) {
	var err error

	// Read in the packages from the package list files.
	var allPackages []string
	for _, packageListRelativePath := range packageLists {
		packageListFilePath := file.GetAbsPathWithBase(baseConfigPath, packageListRelativePath)

		var packageList imagecustomizerapi.PackageList
		err = imagecustomizerapi.UnmarshalYamlFile(packageListFilePath, &packageList)
		if err != nil {
			return nil, fmt.Errorf("failed to read package list file (%s):\n%w", packageListFilePath, err)
		}

		allPackages = append(allPackages, packageList.Packages...)
	}

	allPackages = append(allPackages, packages...)
	return allPackages, nil
}

func removePackages(allPackagesToRemove []string, imageChroot *safechroot.Chroot) error {
	logger.Log.Infof("Removing packages: %v", allPackagesToRemove)

	tdnfRemoveArgs := []string{
		"-v", "remove", "--assumeyes", "--disablerepo", "*",
		// Placeholder for package name.
		"",
	}

	// Remove packages.
	// Do this one at a time, to avoid running out of memory.
	for _, packageName := range allPackagesToRemove {
		tdnfRemoveArgs[len(tdnfRemoveArgs)-1] = packageName

		err := callTdnf(tdnfRemoveArgs, tdnfRemovePrefix, imageChroot)
		if err != nil {
			return fmt.Errorf("failed to remove package (%s):\n%w", packageName, err)
		}
	}

	return nil
}

func updateAllPackages(imageChroot *safechroot.Chroot) error {
	logger.Log.Infof("Updating base image packages")

	tdnfUpdateArgs := []string{
		"-v", "update", "--nogpgcheck", "--assumeyes", "--cacheonly",
		"--setopt", fmt.Sprintf("reposdir=%s", rpmsMountParentDirInChroot),
	}

	err := callTdnf(tdnfUpdateArgs, tdnfInstallPrefix, imageChroot)
	if err != nil {
		return fmt.Errorf("failed to update packages:\n%w", err)
	}

	return nil
}

func installOrUpdatePackages(action string, allPackagesToAdd []string, imageChroot *safechroot.Chroot) error {
	// Create tdnf command args.
	// Note: When using `--repofromdir`, tdnf will not use any default repos and will only use the last
	// `--repofromdir` specified.
	tdnfInstallArgs := []string{
		"-v", action, "--nogpgcheck", "--assumeyes", "--cacheonly",
		"--setopt", fmt.Sprintf("reposdir=%s", rpmsMountParentDirInChroot),
		// Placeholder for package name.
		"",
	}

	// Install packages.
	// Do this one at a time, to avoid running out of memory.
	for _, packageName := range allPackagesToAdd {
		tdnfInstallArgs[len(tdnfInstallArgs)-1] = packageName

		err := callTdnf(tdnfInstallArgs, tdnfInstallPrefix, imageChroot)
		if err != nil {
			return fmt.Errorf("failed to %s package (%s):\n%w", action, packageName, err)
		}
	}

	return nil
}

func callTdnf(tdnfArgs []string, tdnfMessagePrefix string, imageChroot *safechroot.Chroot) error {
	seenTransactionErrorMessage := false
	stdoutCallback := func(line string) {
		if !seenTransactionErrorMessage {
			// Check if this line marks the start of a transaction error message.
			seenTransactionErrorMessage = tdnfTransactionError.MatchString(line)
		}

		if seenTransactionErrorMessage {
			// Report all of the transaction error message (i.e. the remainder of stdout) to WARN.
			logger.Log.Warn(line)
		} else if strings.HasPrefix(line, tdnfMessagePrefix) {
			logger.Log.Debug(line)
		} else {
			logger.Log.Trace(line)
		}
	}

	return imageChroot.UnsafeRun(func() error {
		return shell.NewExecBuilder("tdnf", tdnfArgs...).
			StdoutCallback(stdoutCallback).
			LogLevel(shell.LogDisabledLevel, logrus.DebugLevel).
			ErrorStderrLines(1).
			Execute()
	})
}

func isPackageInstalled(imageChroot *safechroot.Chroot, packageName string) bool {

	err := imageChroot.UnsafeRun(func() error {
		_, _, err := shell.Execute("tdnf", "info", packageName, "--repo", "@system")
		return err
	})
	if err != nil {
		return false
	}
	return true
}

func parseReleaseString(releaseInfo string) (PackageRelease uint32, DistroName string, DistroVersion uint32, err error) {
	pattern := `([0-9]+)\.([a-zA-Z]+)([0-9]+)`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(releaseInfo)

	if matches == nil {
		return 0, "", 0, fmt.Errorf("failed to parse package release information (%s)\n%w", releaseInfo, err)
	}

	// package release
	packageReleaseString := matches[1]
	packageReleaseUint64, err := strconv.ParseUint(packageReleaseString, 10 /*base*/, 32 /*size*/)
	if err != nil {
		return 0, "", 0, fmt.Errorf("failed to parse package release version (%s) into an unsigned integer:\n%w", packageReleaseString, err)
	}
	PackageRelease = uint32(packageReleaseUint64)

	// distro name
	DistroName = matches[2]

	// distro version
	distroVersionString := matches[3]
	distroVersionUint64, err := strconv.ParseUint(distroVersionString, 10 /*base*/, 32 /*size*/)
	if err != nil {
		return 0, "", 0, fmt.Errorf("failed to parse distro version (%s) into an unsigned integer:\n%w", distroVersionString, err)
	}
	DistroVersion = uint32(distroVersionUint64)

	return PackageRelease, DistroName, DistroVersion, nil
}

func parseVersionString(version string) ([]uint64, error) {
	// Regular expression to capture version components
	// Expected patterns are: "number(.number)*"
	re := regexp.MustCompile(`^(\d+)(?:\.(\d+))*$`)

	// Match the version string against the regex
	matches := re.FindStringSubmatch(version)
	if matches == nil {
		return nil, fmt.Errorf("invalid version format: %s", version)
	}

	// Extract all captured groups
	var versionComponents []uint64
	for _, match := range matches[1:] {
		if match == "" {
			continue
		}

		versionComponent, err := strconv.ParseUint(match, 10 /*base*/, 64 /*size*/)
		if err != nil {
			return nil, fmt.Errorf("failed to parse package version component (%s) into an unsigned integer:\n%w", match, err)
		}
		versionComponents = append(versionComponents, versionComponent)
	}

	return versionComponents, nil
}

func getPackageInformation(imageChroot *safechroot.Chroot, packageName string) (info *PackageVersionInformation, err error) {
	var packageInfo string
	err = imageChroot.UnsafeRun(func() error {
		packageInfo, _, err = shell.Execute("tdnf", "info", packageName, "--repo", "@system")
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query (%s) package information:\n%w", packageName, err)
	}

	// Regular expressions to match Version and Release
	versionRegex := regexp.MustCompile(`(?m)^Version\s+:\s+(\S+)`)
	versionMatch := versionRegex.FindStringSubmatch(packageInfo)
	var packageVersion string
	if len(versionMatch) != 2 {
		return nil, fmt.Errorf("failed to extract version information from the (%s) package information (\n%s\n):\n%w", packageName, packageInfo, err)
	}
	packageVersion = versionMatch[1]

	versionComponents, err := parseVersionString(packageVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the (%s) package version information:\n%w", packageName, err)
	}

	// Extract Release
	releaseRegex := regexp.MustCompile(`(?m)^Release\s+:\s+(\S+)`)
	releaseMatch := releaseRegex.FindStringSubmatch(packageInfo)
	var releaseInfo string
	if len(releaseMatch) != 2 {
		return nil, fmt.Errorf("failed to extract release information from the (%s) package information (\n%s\n):\n%w", packageName, packageInfo, err)
	}
	releaseInfo = releaseMatch[1]

	PackageRelease, DistroName, DistroVersion, err := parseReleaseString(releaseInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to parse release information for package (%s)\n%w", packageName, err)
	}

	// Set return values
	info = &PackageVersionInformation{
		PackageVersionComponents: versionComponents,
		PackageRelease:           PackageRelease,
		DistroName:               DistroName,
		DistroVersion:            DistroVersion,
	}

	return info, nil
}

func verifyMinimumVersion(packageName string, packageInfo *PackageVersionInformation, minimumVersionInfo *PackageVersionInformation) error {
	if packageInfo == nil {
		return fmt.Errorf("no package information provided")
	}

	if packageInfo.DistroName != minimumVersionInfo.DistroName {
		return fmt.Errorf("did not find required Azure Linux distro (%s) - found (%s)", minimumVersionInfo.DistroName, packageInfo.DistroName)
	}

	if packageInfo.DistroVersion < minimumVersionInfo.DistroVersion {
		return fmt.Errorf("did not find required Azure Linux distro version (%d) - found (%d)", minimumVersionInfo.DistroVersion, packageInfo.DistroVersion)
	}

	// Note that, theoretically, a newer distro version could still have an older package version.
	// So, it is not sufficient to check that packageInfo.DistroVersion > MinDistroVersion.
	// We need to check the package version number.
	expectedVersion := fmt.Sprintf("%d-%d.%s%d", minimumVersionInfo.PackageVersionComponents[0], minimumVersionInfo.PackageRelease,
		minimumVersionInfo.DistroName, minimumVersionInfo.DistroVersion)

	if len(minimumVersionInfo.PackageVersionComponents) != len(packageInfo.PackageVersionComponents) {
		return fmt.Errorf("unexpected number of version components (%d) for the (%s) package - found (%d)",
			len(minimumVersionInfo.PackageVersionComponents), packageName, len(packageInfo.PackageVersionComponents))
	}

	currentVersion := fmt.Sprintf("%d-%d.%s%d", packageInfo.PackageVersionComponents[0], packageInfo.PackageRelease, packageInfo.DistroName, packageInfo.DistroVersion)

	for i, versionComponent := range packageInfo.PackageVersionComponents {
		if versionComponent < minimumVersionInfo.PackageVersionComponents[i] {
			return fmt.Errorf("did not find required package version (%s) (or newer) - found (%s)", expectedVersion, currentVersion)
		} else if versionComponent > minimumVersionInfo.PackageVersionComponents[i] {
			return nil
		}
	}

	if packageInfo.PackageRelease < minimumVersionInfo.PackageRelease {
		return fmt.Errorf("did not find required package version (%s) (or newer) - found (%s)",
			expectedVersion, currentVersion)
	}

	return nil
}

func getPackageInformationFromRootfsDir(rootfsSourceDir string, packageName string) (info *PackageVersionInformation, err error) {
	chroot := safechroot.NewChroot(rootfsSourceDir, true /*isExistingDir*/)
	if chroot == nil {
		return info, fmt.Errorf("failed to create a new chroot object for %s.", rootfsSourceDir)
	}
	defer chroot.Close(true /*leaveOnDisk*/)

	err = chroot.Initialize("", nil, nil, true /*includeDefaultMounts*/)
	if err != nil {
		return info, fmt.Errorf("failed to initialize chroot object for %s:\n%w", rootfsSourceDir, err)
	}

	return getPackageInformation(chroot, packageName)
}

func cleanTdnfCache(imageChroot *safechroot.Chroot) error {
	logger.Log.Infof("Cleaning up RPM cache")
	// Run all cleanup tasks inside the chroot environment
	return imageChroot.UnsafeRun(func() error {
		tdnfArgs := []string{
			"-v", "clean", "all",
		}
		err := shell.NewExecBuilder("tdnf", tdnfArgs...).
			LogLevel(logrus.TraceLevel, logrus.DebugLevel).
			ErrorStderrLines(1).
			Execute()
		if err != nil {
			return fmt.Errorf("Failed to clean tdnf cache: %w", err)
		}
		return nil
	})
}
