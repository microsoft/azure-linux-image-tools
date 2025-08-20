// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"regexp"
	"slices"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
)

// TDNF Package Manager Implementation
type tdnfPackageManager struct {
	version string
}

func newTdnfPackageManager(version string) *tdnfPackageManager {
	if version == "" {
		version = "3.0" // default version for Azure Linux
	}
	return &tdnfPackageManager{version: version}
}

func (pm *tdnfPackageManager) getPackageManagerBinary() string { return "tdnf" }
func (pm *tdnfPackageManager) getPackageType() PackageType     { return packageTypeRPM }
func (pm *tdnfPackageManager) getReleaseVersion() string       { return pm.version }
func (pm *tdnfPackageManager) getConfigFile() string           { return customTdnfConfRelPath }
func (pm *tdnfPackageManager) getPackageSourceDir() string     { return rpmsMountParentDirInChroot }

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
