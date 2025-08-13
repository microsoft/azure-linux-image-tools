// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"regexp"
	"slices"
)

const (
	azureLinuxReleaseVerCliArg = "--releasever=3.0"
)

// tdnfPackageManager implements the packageManager interface for TDNF (Azure Linux)
type tdnfPackageManager struct {
	downloadRegex         *regexp.Regexp
	transactionErrorRegex *regexp.Regexp
	summaryLines          []string
	opLines               []string
}

// newTdnfPackageManager creates a new TDNF package manager instance
func newTdnfPackageManager() *tdnfPackageManager {
	return &tdnfPackageManager{
		downloadRegex:         regexp.MustCompile(`^(\S+)\s+\S+\s+\S+ \|\s+\S+\s+\S+\s*$`),
		transactionErrorRegex: regexp.MustCompile(`^Problem (\d+): .*|^  - .*`),
		summaryLines:          []string{"Transaction Summary", "Install Summary"},
		opLines:               []string{"Installing   : ", "Upgrading    : ", "Cleanup      : ", "Verifying    : ", "Removing     : "},
	}
}

// getBinaryName returns the TDNF binary name
func (pm *tdnfPackageManager) getBinaryName() string {
	return "tdnf"
}

// getDownloadRegex returns the regex for parsing TDNF download lines
func (pm *tdnfPackageManager) getDownloadRegex() *regexp.Regexp {
	return pm.downloadRegex
}

// getTransactionErrorRegex returns the regex for detecting TDNF transaction errors
func (pm *tdnfPackageManager) getTransactionErrorRegex() *regexp.Regexp {
	return pm.transactionErrorRegex
}

// getSummaryLines returns the lines that indicate the start of TDNF operation summaries
func (pm *tdnfPackageManager) getSummaryLines() []string {
	return pm.summaryLines
}

// getOpLines returns the prefixes that indicate TDNF operation lines
func (pm *tdnfPackageManager) getOpLines() []string {
	return pm.opLines
}

// appendArgsForToolsChroot modifies TDNF arguments for tools chroot operations
func (pm *tdnfPackageManager) appendArgsForToolsChroot(args []string) []string {
	// Add the release version CLI argument to the TDNF arguments.
	if !slices.Contains(args, azureLinuxReleaseVerCliArg) {
		args = append(args, azureLinuxReleaseVerCliArg)
	}

	// Add the install root argument for install or update operations.
	installRootArg := "--installroot=/" + toolsRootImageDir
	if !slices.Contains(args, installRootArg) {
		args = append(args, installRootArg)
	}

	return args
}

// getInstallCommand returns TDNF command arguments for installing packages
func (pm *tdnfPackageManager) getInstallCommand(packages []string, extraOptions map[string]string) []string {
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

// getRemoveCommand returns TDNF command arguments for removing packages
func (pm *tdnfPackageManager) getRemoveCommand(packages []string, extraOptions map[string]string) []string {
	args := []string{"remove", "--assumeyes", "--disablerepo", "*"}
	args = append(args, packages...)
	return args
}

// getUpdateAllCommand returns TDNF command arguments for updating all packages
func (pm *tdnfPackageManager) getUpdateAllCommand(extraOptions map[string]string) []string {
	args := []string{"update", "--assumeyes"}

	if cacheOnly, ok := extraOptions["cacheonly"]; ok && cacheOnly == "true" {
		args = append(args, "--cacheonly")
	}

	if reposDir, ok := extraOptions["reposdir"]; ok {
		args = append(args, "--setopt", "reposdir="+reposDir)
	}

	return args
}

// getUpdateCommand returns TDNF command arguments for updating specific packages
func (pm *tdnfPackageManager) getUpdateCommand(packages []string, extraOptions map[string]string) []string {
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

// getCleanCommand returns TDNF command arguments for cleaning cache
func (pm *tdnfPackageManager) getCleanCommand() []string {
	return []string{"clean", "all"}
}

// getRefreshMetadataCommand returns TDNF command arguments for refreshing metadata
func (pm *tdnfPackageManager) getRefreshMetadataCommand(extraOptions map[string]string) []string {
	args := []string{"makecache"}

	if reposDir, ok := extraOptions["reposdir"]; ok {
		args = append(args, "--setopt", "reposdir="+reposDir)
	}

	return args
}

// requiresMetadataRefresh returns true if TDNF requires explicit metadata refresh
func (pm *tdnfPackageManager) requiresMetadataRefresh() bool {
	return true
}

// supportsCacheOnly returns true as TDNF supports cache-only operations
func (pm *tdnfPackageManager) supportsCacheOnly() bool {
	return true
}

// supportsRepoConfiguration returns true if TDNF supports repository configuration
func (pm *tdnfPackageManager) supportsRepoConfiguration() bool {
	return true
}
