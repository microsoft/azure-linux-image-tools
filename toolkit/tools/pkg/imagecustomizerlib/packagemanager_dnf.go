// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"regexp"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
)

// DNF Package Manager Implementation
type dnfPackageManager struct {
	version string
}

func newDnfPackageManager(version string) *dnfPackageManager {
	return &dnfPackageManager{version: version}
}

func (pm *dnfPackageManager) getPackageManagerBinary() string { return "dnf" }
func (pm *dnfPackageManager) getReleaseVersion() string       { return pm.version }
func (pm *dnfPackageManager) getConfigFile() string           { return "etc/dnf/dnf.conf" }

// getCacheOnlyOptions returns DNF-specific cache options for install/update operations
func (pm *dnfPackageManager) getCacheOnlyOptions() []string {
	return []string{"--setopt=cacheonly=metadata"}
}

// DNF-specific constants and output handling
const (
	dnfDownloadPattern = `^\s*([a-zA-Z0-9\-._+]+(?:\.[a-zA-Z0-9_]+)*\.rpm)\s+.*\d+.*[kMG]B/s.*\|\s*\d+`
)

func (pm *dnfPackageManager) createOutputCallback() func(string) {
	dnfDownloadRegex := regexp.MustCompile(dnfDownloadPattern)

	lastDownloadPackageSeen := ""
	inTransactionSummary := false
	inInstallSection := false
	inUpgradeSection := false
	inRemoveSection := false
	seenTransactionErrorMessage := false
	transactionStarted := false

	return func(line string) {
		trimmedLine := strings.TrimSpace(line)

		// Check for transaction errors first
		if !seenTransactionErrorMessage {
			if strings.HasPrefix(trimmedLine, "Error:") || strings.HasPrefix(trimmedLine, "Problem:") {
				seenTransactionErrorMessage = true
				logger.Log.Warn(line)
				return
			}
		}

		// If we've seen an error, log everything as warning
		if seenTransactionErrorMessage {
			logger.Log.Warn(line)
			return
		}

		switch {
		// DNF Transaction Summary section
		case strings.Contains(trimmedLine, "Transaction Summary"):
			inTransactionSummary = true
			logger.Log.Debug(line)

		case inTransactionSummary && trimmedLine == "":
			inTransactionSummary = false
			logger.Log.Trace(line)

		case inTransactionSummary:
			logger.Log.Debug(line)

		// DNF package operations during transaction
		case strings.HasPrefix(trimmedLine, "Installing :"):
			transactionStarted = true
			logger.Log.Debug(line)

		case strings.HasPrefix(trimmedLine, "Upgrading  :"):
			transactionStarted = true
			logger.Log.Debug(line)

		case strings.HasPrefix(trimmedLine, "Removing   :"):
			transactionStarted = true
			logger.Log.Debug(line)

		case strings.HasPrefix(trimmedLine, "Running scriptlet:"):
			if transactionStarted {
				logger.Log.Debug(line)
			} else {
				logger.Log.Trace(line)
			}

		case strings.HasPrefix(trimmedLine, "Verifying  :"):
			logger.Log.Debug(line)

		// DNF download progress
		case strings.Contains(trimmedLine, "MB/s") && (strings.Contains(trimmedLine, ".rpm") || strings.Contains(trimmedLine, "kB")):
			match := dnfDownloadRegex.FindStringSubmatch(line)
			if match != nil && len(match) > 1 {
				packageName := match[1]
				if packageName != lastDownloadPackageSeen {
					lastDownloadPackageSeen = packageName
					logger.Log.Debug(line)
				}
			} else {
				logger.Log.Debug(line)
			}

		// DNF dependency resolution
		case strings.HasPrefix(trimmedLine, "Dependencies resolved."):
			logger.Log.Debug(line)

		case strings.HasPrefix(trimmedLine, "Installing dependencies:"):
			inInstallSection = true
			logger.Log.Debug(line)

		case strings.HasPrefix(trimmedLine, "Installing weak dependencies:"):
			inInstallSection = true
			logger.Log.Debug(line)

		case strings.HasPrefix(trimmedLine, "Upgrading:"):
			inUpgradeSection = true
			logger.Log.Debug(line)

		case strings.HasPrefix(trimmedLine, "Removing:"):
			inRemoveSection = true
			logger.Log.Debug(line)

		// Package lists in dependency sections
		case (inInstallSection || inUpgradeSection || inRemoveSection) && trimmedLine == "":
			inInstallSection = false
			inUpgradeSection = false
			inRemoveSection = false
			logger.Log.Trace(line)

		case (inInstallSection || inUpgradeSection || inRemoveSection):
			logger.Log.Debug(line)

		// DNF metadata operations
		case strings.Contains(trimmedLine, "metadata") && (strings.Contains(trimmedLine, "downloading") || strings.Contains(trimmedLine, "using")):
			logger.Log.Debug(line)

		case strings.HasPrefix(trimmedLine, "Last metadata expiration check:"):
			logger.Log.Debug(line)

		// DNF progress indicators
		case strings.Contains(trimmedLine, "[") && strings.Contains(trimmedLine, "]") && strings.Contains(trimmedLine, "%"):
			logger.Log.Debug(line)

		// DNF completion messages
		case strings.HasPrefix(trimmedLine, "Complete!"):
			logger.Log.Debug(line)

		case strings.HasPrefix(trimmedLine, "Nothing to do."):
			logger.Log.Debug(line)

		// Default: trace level for other output
		default:
			logger.Log.Trace(line)
		}
	}
}
