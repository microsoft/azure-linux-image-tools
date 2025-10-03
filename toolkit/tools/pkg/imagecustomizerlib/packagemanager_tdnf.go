// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"regexp"
	"slices"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
)

// TDNF Package Manager Implementation
type tdnfPackageManager struct {
	version string
}

func newTdnfPackageManager(version string) *tdnfPackageManager {
	return &tdnfPackageManager{version: version}
}

func (pm *tdnfPackageManager) getPackageManagerBinary() string { return string(packageManagerTDNF) }
func (pm *tdnfPackageManager) getReleaseVersion() string       { return pm.version }
func (pm *tdnfPackageManager) getConfigFile() string           { return customTdnfConfRelPath }

// getVerbosityOption returns the package manager-specific verbosity flag
func (pm *tdnfPackageManager) getVerbosityOption() string { return "-v" }

// getCacheOnlyOptions returns TDNF-specific cache options for install/update operations
func (pm *tdnfPackageManager) getCacheOnlyOptions() []string {
	return nil // TDNF doesn't need additional cache options
}

// supportsSnapshotTime returns whether TDNF supports snapshot time functionality
func (pm *tdnfPackageManager) supportsSnapshotTime() bool {
	return true // TDNF supports snapshot time for Azure Linux
}

// TDNF-specific constants and output handling
const (
	tdnfTransactionErrorPattern = `^Found \d+ problems$`
	tdnfDownloadPattern         = `^\s*([a-zA-Z0-9\-._+]+)\s+\d+\%\s+\d+$`
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

	tdnfTransactionErrorRegex = regexp.MustCompile(tdnfTransactionErrorPattern)
	tdnfDownloadRegex         = regexp.MustCompile(tdnfDownloadPattern)
)

func (pm *tdnfPackageManager) createOutputCallback() func(string) {
	lastDownloadPackageSeen := ""
	inSummary := false
	seenTransactionErrorMessage := false

	return func(line string) {
		if !seenTransactionErrorMessage {
			seenTransactionErrorMessage = tdnfTransactionErrorRegex.MatchString(line)
		}

		switch {
		case seenTransactionErrorMessage:
			logger.Log.Warn(line)

		case inSummary && line == "":
			inSummary = false
			logger.Log.Trace(line)

		case inSummary:
			logger.Log.Debug(line)

		case slices.Contains(tdnfSummaryLines, line):
			inSummary = true
			logger.Log.Debug(line)

		case slices.ContainsFunc(tdnfOpLines, func(opPrefix string) bool { return strings.HasPrefix(line, opPrefix) }):
			logger.Log.Debug(line)

		default:
			match := tdnfDownloadRegex.FindStringSubmatch(line)
			if match != nil {
				packageName := match[1]
				if packageName != lastDownloadPackageSeen {
					lastDownloadPackageSeen = packageName
					logger.Log.Debug(line)
					return
				}
			}
			logger.Log.Trace(line)
		}
	}
}
