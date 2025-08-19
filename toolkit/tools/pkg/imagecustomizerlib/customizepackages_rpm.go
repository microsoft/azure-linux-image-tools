package imagecustomizerlib

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
)

// TDNF-specific constants
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

	// Download log message for TDNF.
	// For example:
	//   jq 6% 15709
	tdnfDownloadRegex = regexp.MustCompile(tdnfDownloadPattern)
)

// DNF-specific constants
// Download log message for DNF.
// For example:
//
//	curl-7.68.0-1.fc42.x86_64.rpm                   1.2 MB/s | 355 kB     00:00
const (
	dnfDownloadPattern = `^\s*([a-zA-Z0-9\-._+]+(?:\.[a-zA-Z0-9_]+)*\.rpm)\s+.*\d+.*[kMG]B/s.*\|\s*\d+`
)

// azureLinuxDistroConfig implements distroHandler for Azure Linux
type azureLinuxDistroConfig struct {
	version string
}

func (d *azureLinuxDistroConfig) getPackageManagerBinary() string { return "tdnf" }
func (d *azureLinuxDistroConfig) getPackageType() PackageType     { return packageTypeRPM }
func (d *azureLinuxDistroConfig) getReleaseVersion() string {
	if d.version != "" {
		return d.version
	}
	return "3.0" // default version
}
func (d *azureLinuxDistroConfig) getConfigFile() string       { return customTdnfConfRelPath }
func (d *azureLinuxDistroConfig) getDistroName() DistroName   { return distroNameAzureLinux }
func (d *azureLinuxDistroConfig) getPackageSourceDir() string { return rpmsMountParentDirInChroot }

// Distribution-specific logging and output handling for Azure Linux (TDNF)
func (d *azureLinuxDistroConfig) createOutputCallback() func(string) {
	lastDownloadPackageSeen := ""
	inSummary := false
	seenTransactionErrorMessage := false

	return func(line string) {
		if !seenTransactionErrorMessage {
			// Check if this line marks the start of a transaction error message.
			seenTransactionErrorMessage = tdnfTransactionErrorRegex.MatchString(line)
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
					return
				}
			}

			logger.Log.Trace(line)
		}
	}
}

// fedoraDistroConfig implements distroHandler for Fedora
type fedoraDistroConfig struct {
	version string
}

func (d *fedoraDistroConfig) getPackageManagerBinary() string { return "dnf" }
func (d *fedoraDistroConfig) getPackageType() PackageType     { return packageTypeRPM }
func (d *fedoraDistroConfig) getReleaseVersion() string {
	if d.version != "" {
		return d.version
	}
	return "42" // default version
}
func (d *fedoraDistroConfig) getConfigFile() string       { return "etc/dnf/dnf.conf" }
func (d *fedoraDistroConfig) getDistroName() DistroName   { return distroNameFedora }
func (d *fedoraDistroConfig) getPackageSourceDir() string { return rpmsMountParentDirInChroot }

// Distribution-specific logging and output handling for Fedora (DNF)
func (d *fedoraDistroConfig) createOutputCallback() func(string) {
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
			// End of transaction summary
			inTransactionSummary = false
			logger.Log.Trace(line)

		case inTransactionSummary:
			// Inside transaction summary
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
			// DNF download format: package.rpm    size MB/s | total size    time
			match := dnfDownloadRegex.FindStringSubmatch(line)
			if match != nil && len(match) > 1 {
				packageName := match[1]
				if packageName != lastDownloadPackageSeen {
					lastDownloadPackageSeen = packageName
					logger.Log.Debug(line)
				}
			} else {
				logger.Log.Debug(line) // Log download lines even if regex doesn't match
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
			// End of section
			inInstallSection = false
			inUpgradeSection = false
			inRemoveSection = false
			logger.Log.Trace(line)

		case (inInstallSection || inUpgradeSection || inRemoveSection):
			// Package name in dependency section
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

// managePackagesRpm provides a shared implementation for RPM-based package management
func managePackagesRpm(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.OS,
	imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	rpmsSources []string, useBaseImageRpmRepos bool, snapshotTime string, distroConfig distroHandler,
) error {
	var err error

	packageManagerChroot := imageChroot
	if toolsChroot != nil {
		packageManagerChroot = toolsChroot
	}

	// Setup distribution-specific configuration if needed
	if distroConfig.getDistroName() == distroNameAzureLinux {
		// Setup Azure Linux specific TDNF configuration with snapshot
		err = createTempTdnfConfigWithSnapshot(packageManagerChroot, imagecustomizerapi.PackageSnapshotTime(snapshotTime))
		if err != nil {
			return err
		}
		defer func() {
			if cleanupErr := cleanupSnapshotTimeConfig(packageManagerChroot); cleanupErr != nil && err == nil {
				err = cleanupErr
			}
		}()
	}

	needPackageSources := len(config.Packages.Install) > 0 || len(config.Packages.Update) > 0 ||
		config.Packages.UpdateExistingPackages

	var mounts *rpmSourcesMounts
	if needPackageSources {
		// Mount RPM sources
		mounts, err = mountRpmSources(ctx, buildDir, packageManagerChroot, rpmsSources, useBaseImageRpmRepos)
		if err != nil {
			return err
		}
		defer mounts.close()

		// Refresh metadata
		err = refreshPackageMetadata(ctx, imageChroot, toolsChroot, distroConfig)
		if err != nil {
			return err
		}

		// Prefill cache for Fedora
		if distroConfig.getDistroName() == distroNameFedora {
			err = prefillDnfCache(ctx, config.Packages.Install, config.Packages.Update, imageChroot, toolsChroot, distroConfig)
			if err != nil {
				return err
			}
		}
	}

	// Execute package operations
	err = removePackages(ctx, config.Packages.Remove, imageChroot, toolsChroot, distroConfig)
	if err != nil {
		return err
	}

	if config.Packages.UpdateExistingPackages {
		err = updateAllPackages(ctx, imageChroot, toolsChroot, distroConfig)
		if err != nil {
			return err
		}
	}

	err = installOrUpdatePackages(ctx, "install", config.Packages.Install, imageChroot, toolsChroot, distroConfig)
	if err != nil {
		return err
	}

	err = installOrUpdatePackages(ctx, "update", config.Packages.Update, imageChroot, toolsChroot, distroConfig)
	if err != nil {
		return err
	}

	// Cleanup
	if mounts != nil {
		err = mounts.close()
		if err != nil {
			return err
		}
	}

	if needPackageSources {
		err = cleanCache(imageChroot, toolsChroot, distroConfig)
		if err != nil {
			return err
		}
	}

	return nil
}

// managePackages handles the complete package management workflow for Azure Linux
func (d *azureLinuxDistroConfig) managePackages(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.OS,
	imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	rpmsSources []string, useBaseImageRpmRepos bool, snapshotTime string,
) error {
	return managePackagesRpm(ctx, buildDir, baseConfigPath, config, imageChroot, toolsChroot, rpmsSources, useBaseImageRpmRepos, snapshotTime, d)
}

// managePackages handles the complete package management workflow for Fedora
func (d *fedoraDistroConfig) managePackages(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.OS,
	imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	rpmsSources []string, useBaseImageRpmRepos bool, snapshotTime string,
) error {
	return managePackagesRpm(ctx, buildDir, baseConfigPath, config, imageChroot, toolsChroot, rpmsSources, useBaseImageRpmRepos, snapshotTime, d)
}

// prefillDnfCache downloads packages to DNF cache for offline installation
func prefillDnfCache(ctx context.Context, installPackages []string, updatePackages []string, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, distroConfig distroHandler) error {
	allPackages := append(installPackages, updatePackages...)
	if len(allPackages) == 0 {
		return nil
	}

	logger.Log.Infof("Pre-filling DNF cache for packages: %v", allPackages)

	args := []string{"-v", "install", "--assumeyes", "--downloadonly"}

	repoDir := distroConfig.getPackageSourceDir()
	if repoDir != "" {
		args = append(args, "--setopt=reposdir="+repoDir)
	}

	// Enable keepcache to ensure packages are stored in cache
	args = append(args, "--setopt=keepcache=1")

	if toolsChroot != nil {
		args = append([]string{"--releasever=" + distroConfig.getReleaseVersion(), "--installroot=/" + toolsRootImageDir}, args...)
	}

	args = append(args, allPackages...)

	err := executePackageManagerCommand(args, imageChroot, toolsChroot, distroConfig)
	if err != nil {
		return fmt.Errorf("failed to prefill DNF cache for packages (%v):\n%w", allPackages, err)
	}
	return nil
}
