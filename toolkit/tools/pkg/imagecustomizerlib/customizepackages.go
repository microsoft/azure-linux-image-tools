// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/sirupsen/logrus"
)

// TODO: Remove this constant to support 4.0 and later releases.
const (
	releaseVerCliArg = "--releasever=3.0"
)

var (
	tdnfOpLines = []string{
		"Installing/Updating: ",
		"Removing: ",
	}

	tdnfSummaryLines = []string{
		"Installing:",
		"Upgrading:",
		"Removing:",
	}

	tdnfTransactionError = regexp.MustCompile(`^Found \d+ problems$`)

	// Download log message.
	// For example:
	//   jq 6% 15709
	tdnfDownloadRegex = regexp.MustCompile(`^\s*([a-zA-Z0-9\-._+]+)\s+\d+\%\s+\d+$`)
)

func addRemoveAndUpdatePackages(buildDir string, baseConfigPath string, config *imagecustomizerapi.OS,
	imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	rpmsSources []string, useBaseImageRpmRepos bool, snapshotTime string,
) error {
	var err error

	if snapshotTime == "" {
		snapshotTime = string(config.Packages.SnapshotTime)
	}

	tdnfChroot := imageChroot
	if toolsChroot != nil {
		tdnfChroot = toolsChroot
	}

	err = createTempTdnfConfigWithSnapshot(tdnfChroot, imagecustomizerapi.PackageSnapshotTime(snapshotTime))
	if err != nil {
		return err
	}
	defer func() {
		if cleanupErr := cleanupSnapshotTimeConfig(tdnfChroot); cleanupErr != nil && err == nil {
			err = cleanupErr
		}
	}()

	// Note: The 'validatePackageLists' function read the PackageLists files and merged them into the inline package lists.
	needRpmsSources := len(config.Packages.Install) > 0 || len(config.Packages.Update) > 0 ||
		config.Packages.UpdateExistingPackages

	var mounts *rpmSourcesMounts
	if needRpmsSources {
		// Mount RPM sources.
		mounts, err = mountRpmSources(buildDir, tdnfChroot, rpmsSources, useBaseImageRpmRepos)
		if err != nil {
			return err
		}
		defer mounts.close()

		// Refresh metadata.
		err = refreshTdnfMetadata(imageChroot, toolsChroot)
		if err != nil {
			return err
		}
	}

	err = removePackages(config.Packages.Remove, imageChroot, toolsChroot)
	if err != nil {
		return err
	}

	if config.Packages.UpdateExistingPackages {
		err = updateAllPackages(imageChroot, toolsChroot)
		if err != nil {
			return err
		}
	}

	logger.Log.Infof("Installing packages: %v", config.Packages.Install)
	err = installOrUpdatePackages("install", config.Packages.Install, imageChroot, toolsChroot)
	if err != nil {
		return err
	}

	logger.Log.Infof("Updating packages: %v", config.Packages.Update)
	err = installOrUpdatePackages("update", config.Packages.Update, imageChroot, toolsChroot)
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
		err = cleanTdnfCache(imageChroot, toolsChroot)
		if err != nil {
			return err
		}
	}

	return nil
}

func refreshTdnfMetadata(imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot) error {
	tdnfArgs := []string{
		"-v", "check-update", "--refresh", "--assumeyes",
		"--setopt", fmt.Sprintf("reposdir=%s", rpmsMountParentDirInChroot),
	}

	tdnfChroot := imageChroot
	if toolsChroot != nil {
		tdnfArgs = appendTdnfArgsForToolsChroot(tdnfArgs)
		tdnfChroot = toolsChroot
	}

	err := tdnfChroot.UnsafeRun(func() error {
		return shell.NewExecBuilder("tdnf", tdnfArgs...).
			LogLevel(logrus.TraceLevel, logrus.DebugLevel).
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
		err = imagecustomizerapi.UnmarshalAndValidateYamlFile(packageListFilePath, &packageList)
		if err != nil {
			return nil, fmt.Errorf("failed to read package list file (%s):\n%w", packageListFilePath, err)
		}

		allPackages = append(allPackages, packageList.Packages...)
	}

	allPackages = append(allPackages, packages...)
	return allPackages, nil
}

func removePackages(allPackagesToRemove []string, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot) error {
	logger.Log.Infof("Removing packages: %v", allPackagesToRemove)

	if len(allPackagesToRemove) <= 0 {
		return nil
	}

	tdnfRemoveArgs := []string{
		"-v", "remove", "--assumeyes", "--disablerepo", "*",
	}

	tdnfRemoveArgs = append(tdnfRemoveArgs, allPackagesToRemove...)

	err := callTdnf(tdnfRemoveArgs, imageChroot, toolsChroot)
	if err != nil {
		return fmt.Errorf("failed to remove packages (%v):\n%w", allPackagesToRemove, err)
	}

	return nil
}

func updateAllPackages(imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot) error {
	logger.Log.Infof("Updating base image packages")

	tdnfUpdateArgs := []string{
		"-v", "update", "--assumeyes", "--cacheonly",
		"--setopt", fmt.Sprintf("reposdir=%s", rpmsMountParentDirInChroot),
	}

	err := callTdnf(tdnfUpdateArgs, imageChroot, toolsChroot)
	if err != nil {
		return fmt.Errorf("failed to update packages:\n%w", err)
	}

	return nil
}

func installOrUpdatePackages(action string, allPackagesToAdd []string, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot) error {
	if len(allPackagesToAdd) <= 0 {
		return nil
	}

	// Create tdnf command args.
	// Note: When using `--repofromdir`, tdnf will not use any default repos and will only use the last
	// `--repofromdir` specified.
	tdnfInstallArgs := []string{
		"-v", action, "--assumeyes", "--cacheonly",
		"--setopt", fmt.Sprintf("reposdir=%s", rpmsMountParentDirInChroot),
	}

	tdnfInstallArgs = append(tdnfInstallArgs, allPackagesToAdd...)

	err := callTdnf(tdnfInstallArgs, imageChroot, toolsChroot)
	if err != nil {
		return fmt.Errorf("failed to %s packages (%v):\n%w", action, allPackagesToAdd, err)
	}

	return nil
}

func callTdnf(tdnfArgs []string, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot) error {
	tdnfChroot := imageChroot
	if toolsChroot != nil {
		tdnfArgs = appendTdnfArgsForToolsChroot(tdnfArgs)
		tdnfChroot = toolsChroot
	}
	if _, err := os.Stat(filepath.Join(tdnfChroot.RootDir(), customTdnfConfRelPath)); err == nil {
		tdnfArgs = append([]string{"--config", "/" + customTdnfConfRelPath}, tdnfArgs...)
	}

	lastDownloadPackageSeen := ""
	inSummary := false
	seenTransactionErrorMessage := false
	stdoutCallback := func(line string) {
		if !seenTransactionErrorMessage {
			// Check if this line marks the start of a transaction error message.
			seenTransactionErrorMessage = tdnfTransactionError.MatchString(line)
		}

		switch {
		case seenTransactionErrorMessage:
			// Report all of the transaction error message (i.e. the remainder of stdout) to WARN.
			logger.Log.Warn(line)

		case inSummary && line == "":
			// Summary end.
			inSummary = false
			logger.Log.Trace(line)

		case inSummary:
			// Summary continues.
			logger.Log.Debug(line)

		case slices.Contains(tdnfSummaryLines, line):
			// Summary start.
			inSummary = true
			logger.Log.Debug(line)

		case slices.ContainsFunc(tdnfOpLines, func(opPrefix string) bool { return strings.HasPrefix(line, opPrefix) }):
			logger.Log.Debug(line)

		default:
			match := tdnfDownloadRegex.FindStringSubmatch(line)
			if match != nil {
				packageName := match[1]
				if packageName != lastDownloadPackageSeen {
					// Log the download logs. But only log once per package to avoid spamming the debug logs.
					lastDownloadPackageSeen = packageName
					logger.Log.Debug(line)
					break
				}
			}

			logger.Log.Trace(line)
		}
	}

	return tdnfChroot.UnsafeRun(func() error {
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

func cleanTdnfCache(imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot) error {
	logger.Log.Infof("Cleaning up RPM cache")

	tdnfArgs := []string{
		"-v", "clean", "all",
	}

	tdnfChroot := imageChroot
	if toolsChroot != nil {

		tdnfArgs = appendTdnfArgsForToolsChroot(tdnfArgs)
		tdnfChroot = toolsChroot
	}

	// Run all cleanup tasks inside the chroot environment
	return tdnfChroot.UnsafeRun(func() error {
		err := shell.NewExecBuilder("tdnf", tdnfArgs...).
			LogLevel(logrus.TraceLevel, logrus.DebugLevel).
			ErrorStderrLines(1).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to clean tdnf cache:\n%w", err)
		}
		return nil
	})
}

// Update the TDNF arguments to include the release version and install root options for the tools chroot.
func appendTdnfArgsForToolsChroot(tdnfArgs []string) []string {
	// Add the release version CLI argument to the TDNF arguments.
	if !slices.Contains(tdnfArgs, releaseVerCliArg) {
		tdnfArgs = append(tdnfArgs, releaseVerCliArg)
	}

	// Add the install root argument for install or update operations.
	installRootArg := "--installroot=/" + toolsRootImageDir
	if !slices.Contains(tdnfArgs, installRootArg) {
		tdnfArgs = append(tdnfArgs, installRootArg)
	}

	return tdnfArgs
}
