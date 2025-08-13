// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"regexp"
	"slices"
)

const (
	fedoraReleaseVerCliArg = "--releasever=42"
)

// dnfPackageManager implements the packageManager interface for DNF (Fedora)
type dnfPackageManager struct {
	downloadRegex         *regexp.Regexp
	transactionErrorRegex *regexp.Regexp
	summaryLines          []string
	opLines               []string
}

// newDnfPackageManager creates a new DNF package manager instance
func newDnfPackageManager() *dnfPackageManager {
	return &dnfPackageManager{
		downloadRegex:         regexp.MustCompile(`^(\S+)\s+\S+\s+\S+ \|\s+\S+\s+\S+\s*$`),
		transactionErrorRegex: regexp.MustCompile(`^Problem (\d+): .*|^  - .*`),
		summaryLines:          []string{"Transaction Summary", "Install Summary"},
		opLines:               []string{"Installing  : ", "Upgrading   : ", "Cleanup     : ", "Verifying   : ", "Removing    : "},
	}
}

// getBinaryName returns the DNF binary name
func (pm *dnfPackageManager) getBinaryName() string {
	return "dnf"
}

// getDownloadRegex returns the regex for parsing DNF download lines
func (pm *dnfPackageManager) getDownloadRegex() *regexp.Regexp {
	return pm.downloadRegex
}

// getTransactionErrorRegex returns the regex for detecting DNF transaction errors
func (pm *dnfPackageManager) getTransactionErrorRegex() *regexp.Regexp {
	return pm.transactionErrorRegex
}

// getSummaryLines returns the lines that indicate the start of DNF operation summaries
func (pm *dnfPackageManager) getSummaryLines() []string {
	return pm.summaryLines
}

// getOpLines returns the prefixes that indicate DNF operation lines
func (pm *dnfPackageManager) getOpLines() []string {
	return pm.opLines
}

// appendArgsForToolsChroot modifies DNF arguments for tools chroot operations
func (pm *dnfPackageManager) appendArgsForToolsChroot(args []string) []string {
	// Add the release version CLI argument to the DNF arguments for Fedora.
	if !slices.Contains(args, fedoraReleaseVerCliArg) {
		args = append(args, fedoraReleaseVerCliArg)
	}

	// Add the install root argument for install or update operations.
	installRootArg := "--installroot=/" + toolsRootImageDir
	args = append(args, installRootArg)
	return args
}

// getInstallCommand returns DNF command arguments for installing packages
func (pm *dnfPackageManager) getInstallCommand(packages []string, extraOptions map[string]string) []string {
	args := []string{"install", "--assumeyes"}

	if cacheOnly, ok := extraOptions["cacheonly"]; ok && cacheOnly == "true" {
		args = append(args, "--cacheonly")
	}

	if reposDir, ok := extraOptions["reposdir"]; ok {
		args = append(args, "--setopt", "reposdir="+reposDir)
	}

	args = append(args, packages...)
	return args
}

// getRemoveCommand returns DNF command arguments for removing packages
func (pm *dnfPackageManager) getRemoveCommand(packages []string, extraOptions map[string]string) []string {
	args := []string{"remove", "--assumeyes", "--disablerepo", "*"}
	args = append(args, packages...)
	return args
}

// getUpdateAllCommand returns DNF command arguments for updating all packages
func (pm *dnfPackageManager) getUpdateAllCommand(extraOptions map[string]string) []string {
	args := []string{"update", "--assumeyes"}

	if cacheOnly, ok := extraOptions["cacheonly"]; ok && cacheOnly == "true" {
		args = append(args, "--cacheonly")
	}

	if reposDir, ok := extraOptions["reposdir"]; ok {
		args = append(args, "--setopt", "reposdir="+reposDir)
	}

	return args
}

// getUpdateCommand returns DNF command arguments for updating specific packages
func (pm *dnfPackageManager) getUpdateCommand(packages []string, extraOptions map[string]string) []string {
	args := []string{"update", "--assumeyes"}

	if cacheOnly, ok := extraOptions["cacheonly"]; ok && cacheOnly == "true" {
		args = append(args, "--cacheonly")
	}

	if reposDir, ok := extraOptions["reposdir"]; ok {
		args = append(args, "--setopt", "reposdir="+reposDir)
	}

	args = append(args, packages...)
	return args
}

// getCleanCommand returns DNF command arguments for cleaning cache
func (pm *dnfPackageManager) getCleanCommand() []string {
	return []string{"clean", "all"}
}

// getRefreshMetadataCommand returns DNF command arguments for refreshing metadata
func (pm *dnfPackageManager) getRefreshMetadataCommand(extraOptions map[string]string) []string {
	args := []string{"makecache"}

	if reposDir, ok := extraOptions["reposdir"]; ok {
		args = append(args, "--setopt", "reposdir="+reposDir)
	}

	return args
}

// requiresMetadataRefresh returns true if DNF requires explicit metadata refresh
func (pm *dnfPackageManager) requiresMetadataRefresh() bool {
	return true
}

// supportsCacheOnly returns false as DNF needs to have keepcache=1 step run before cacheonly
// to work correctly
func (pm *dnfPackageManager) supportsCacheOnly() bool {
	return false
}

// supportsRepoConfiguration returns true if DNF supports repository configuration
func (pm *dnfPackageManager) supportsRepoConfiguration() bool {
	return true
}
