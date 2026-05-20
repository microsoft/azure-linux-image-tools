// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/sirupsen/logrus"
)

const (
	// Sysext locations as specified in the ACL image layout.
	aclEmbeddedSysextDir = "/usr/share/flatcar/sysext"
	aclOemSysextDir      = "/oem/sysext"

	// Temp directory created inside the toolsChroot for extracting sysext RPM DBs.
	sysextCheckTempDir = "/tmp/ic-sysext-check"
)

// squashfsMagicLE is the on-disk byte sequence for little-endian squashfs (modern x86 format).
// Squashfs stores its 32-bit magic value (0x73717368) in little-endian byte order on disk.
var squashfsMagicLE = [4]byte{0x68, 0x73, 0x71, 0x73}

// squashfsMagicBE is the on-disk byte sequence for big-endian squashfs (legacy format).
var squashfsMagicBE = [4]byte{0x73, 0x71, 0x73, 0x68}

// checkACLSysextConflicts runs three layers of sysext conflict detection before any package
// operations are applied to the ACL image. A conflict on any layer is a hard error.
//
// Layer 1: File-path overlap — files the package would install vs. files provided by each sysext.
// Layer 2: RPM inventory — the package appears in a sysext's RPM DB (install/update/remove).
// Layer 3: Dependency integrity — a sysext package depends on a package being removed.
func checkACLSysextConflicts(
	ctx context.Context,
	buildDir string,
	baseConfigPath string,
	imageChroot *safechroot.Chroot,
	toolsChroot *safechroot.Chroot,
	config *imagecustomizerapi.OS,
	rpmsSources []string,
	useBaseImageRpmRepos bool,
	pmHandler rpmPackageManagerHandler,
) error {
	if toolsChroot == nil {
		return fmt.Errorf("internal error: sysext conflict check requires a tools chroot")
	}

	// Collect the full package lists from config (resolves package list files).
	installPkgs, err := collectPackagesList(baseConfigPath, config.Packages.InstallLists, config.Packages.Install)
	if err != nil {
		return fmt.Errorf("failed to collect install package list for sysext conflict check:\n%w", err)
	}
	updatePkgs, err := collectPackagesList(baseConfigPath, config.Packages.UpdateLists, config.Packages.Update)
	if err != nil {
		return fmt.Errorf("failed to collect update package list for sysext conflict check:\n%w", err)
	}
	removePkgs, err := collectPackagesList(baseConfigPath, config.Packages.RemoveLists, config.Packages.Remove)
	if err != nil {
		return fmt.Errorf("failed to collect remove package list for sysext conflict check:\n%w", err)
	}

	hasInstallOrUpdate := len(installPkgs) > 0 || len(updatePkgs) > 0
	hasRemove := len(removePkgs) > 0
	if !hasInstallOrUpdate && !hasRemove {
		return nil
	}

	// Discover all sysext .raw files accessible in the mounted image.
	sysextFiles, err := discoverACLSysextFiles(imageChroot)
	if err != nil {
		return err
	}
	if len(sysextFiles) == 0 {
		logger.Log.Debugf("No sysext files found; skipping sysext conflict check")
		return nil
	}
	logger.Log.Debugf("Found %d sysext file(s) for conflict check", len(sysextFiles))

	// Create and defer cleanup of the temp extraction directory inside the tools chroot.
	sysextCheckHostDir := filepath.Join(toolsChroot.RootDir(), sysextCheckTempDir)
	if err := os.MkdirAll(sysextCheckHostDir, 0o755); err != nil {
		return fmt.Errorf("failed to create sysext check temp dir:\n%w", err)
	}
	defer os.RemoveAll(sysextCheckHostDir)

	// Layer 1: file-path overlap (install and update only).
	if hasInstallOrUpdate {
		allInstallUpdatePkgs := append(installPkgs, updatePkgs...)
		if err := checkSysextFilePathConflicts(ctx, buildDir, toolsChroot, sysextFiles, allInstallUpdatePkgs,
			rpmsSources, useBaseImageRpmRepos, pmHandler); err != nil {
			return err
		}
	}

	// Layers 2 and 3: extract RPM DBs from all sysexts and run inventory/dependency checks.
	allPkgs := append(append(installPkgs, updatePkgs...), removePkgs...)
	for _, sysextHostPath := range sysextFiles {
		sysextName := strings.TrimSuffix(filepath.Base(sysextHostPath), ".raw")

		// Convert the sysext host path to a chroot-relative path for use inside toolsChroot.
		sysextChrootPath, err := hostPathToToolsChrootPath(toolsChroot, sysextHostPath)
		if err != nil {
			return fmt.Errorf("sysext path not under tools chroot (%s):\n%w", sysextHostPath, err)
		}

		// Extract only the RPM DB from the sysext to a temp dir inside the tools chroot.
		rpmDbChrootDir := filepath.Join(sysextCheckTempDir, sysextName)
		if err := extractSysextRpmDb(toolsChroot, sysextChrootPath, rpmDbChrootDir); err != nil {
			// If the sysext has no RPM DB, layers 2 and 3 don't apply to it.
			logger.Log.Debugf("Sysext (%s) has no RPM DB; skipping inventory and dependency checks: %v",
				sysextName, err)
			continue
		}

		rpmDbChrootPath := filepath.Join(rpmDbChrootDir, "var", "lib", "rpm")

		// Layer 2: RPM package inventory — block if any requested package already exists in the sysext.
		if err := checkSysextRpmInventory(toolsChroot, sysextName, rpmDbChrootPath, allPkgs); err != nil {
			return err
		}

		// Layer 3: dependency integrity — block if any sysext package depends on a package being removed.
		if hasRemove {
			if err := checkSysextDependencyIntegrity(toolsChroot, sysextName, rpmDbChrootPath, removePkgs); err != nil {
				return err
			}
		}
	}

	return nil
}

// discoverACLSysextFiles returns the host-side paths to all .raw sysext files in the ACL image.
func discoverACLSysextFiles(imageChroot *safechroot.Chroot) ([]string, error) {
	var allFiles []string
	for _, dir := range []string{aclEmbeddedSysextDir, aclOemSysextDir} {
		pattern := filepath.Join(imageChroot.RootDir(), dir, "*.raw")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to glob sysext files (%s):\n%w", pattern, err)
		}
		for _, m := range matches {
			isSquashfs, err := isSysextSquashfs(m)
			if err != nil {
				return nil, fmt.Errorf("failed to detect sysext format (%s):\n%w", m, err)
			}
			if !isSquashfs {
				// TODO: add DDI (GPT disk image) support when ACL migrates to DDI-based sysexts.
				// DDI sysexts will have a GPT signature at offset 512 ("EFI PART").
				// For now, emit a warning and skip — fail loud only once the DDI migration lands.
				logger.Log.Warnf("Sysext (%s) is not squashfs; DDI format not yet supported. "+
					"Conflict detection skipped for this sysext.", filepath.Base(m))
				continue
			}
			allFiles = append(allFiles, m)
		}
	}
	return allFiles, nil
}

// isSysextSquashfs returns true if the file at path begins with a known squashfs magic sequence.
func isSysextSquashfs(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	var magic [4]byte
	if _, err := f.Read(magic[:]); err != nil {
		return false, err
	}
	return magic == squashfsMagicLE || magic == squashfsMagicBE, nil
}

// hostPathToToolsChrootPath converts a host-side absolute path that is under the tools chroot
// root into the equivalent path as seen inside the tools chroot.
func hostPathToToolsChrootPath(toolsChroot *safechroot.Chroot, hostPath string) (string, error) {
	rel, err := filepath.Rel(toolsChroot.RootDir(), hostPath)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path (%s) is outside the tools chroot (%s)", hostPath, toolsChroot.RootDir())
	}
	return filepath.Join("/", rel), nil
}

// checkSysextFilePathConflicts implements Layer 1: compare the files each package would install
// against the file list of every squashfs sysext. A match is a hard error.
func checkSysextFilePathConflicts(
	ctx context.Context,
	buildDir string,
	toolsChroot *safechroot.Chroot,
	sysextHostPaths []string,
	packages []string,
	rpmsSources []string,
	useBaseImageRpmRepos bool,
	pmHandler rpmPackageManagerHandler,
) error {
	// Mount RPM sources so tdnf repoquery can access repo metadata.
	mounts, err := mountRpmSources(ctx, buildDir, toolsChroot, rpmsSources, useBaseImageRpmRepos)
	if err != nil {
		return fmt.Errorf("failed to mount RPM sources for sysext conflict check:\n%w", err)
	}
	defer mounts.close()

	// Refresh repo metadata so repoquery has accurate package file lists.
	if err := refreshRpmPackageMetadata(ctx, nil, toolsChroot, pmHandler); err != nil {
		return fmt.Errorf("failed to refresh repo metadata for sysext conflict check:\n%w", err)
	}

	// Build the combined sysext file set once (across all sysexts, keyed by path → sysext name).
	sysextPathToSysext := make(map[string]string)
	for _, hostPath := range sysextHostPaths {
		sysextName := strings.TrimSuffix(filepath.Base(hostPath), ".raw")
		sysextChrootPath, err := hostPathToToolsChrootPath(toolsChroot, hostPath)
		if err != nil {
			return err
		}
		fileList, err := listSysextFiles(toolsChroot, sysextChrootPath)
		if err != nil {
			return fmt.Errorf("failed to list files in sysext (%s):\n%w", sysextName, err)
		}
		for f := range fileList {
			sysextPathToSysext[f] = sysextName
		}
	}

	// For each package, get its file list and check for overlap.
	for _, pkg := range packages {
		pkgFiles, err := tdnfRepoqueryList(toolsChroot, pkg, pmHandler)
		if err != nil {
			return fmt.Errorf("failed to query files for package (%s):\n%w", pkg, err)
		}
		for _, f := range pkgFiles {
			if sysextName, conflict := sysextPathToSysext[f]; conflict {
				return fmt.Errorf(
					"package '%s' would install file '%s' which is already provided by sysext '%s':\n"+
						"the base image change would be silently shadowed by the sysext overlay at runtime;\n"+
						"remove or replace the conflicting sysext before running image customizer",
					pkg, f, sysextName)
			}
		}
	}
	return nil
}

// listSysextFiles runs unsquashfs -l inside the tools chroot to list all file paths in a
// squashfs sysext. Returns a set of absolute paths (e.g. "/usr/bin/foo").
func listSysextFiles(toolsChroot *safechroot.Chroot, sysextChrootPath string) (map[string]bool, error) {
	stdout, _, err := shell.NewExecBuilder("unsquashfs", "-l", sysextChrootPath).
		LogLevel(logrus.TraceLevel, logrus.DebugLevel).
		Chroot(toolsChroot.ChrootDir()).
		ExecuteCaptureOutput()
	if err != nil {
		return nil, fmt.Errorf("unsquashfs -l failed for (%s):\n%w", sysextChrootPath, err)
	}

	files := make(map[string]bool)
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "squashfs-root") {
			continue
		}
		// Strip "squashfs-root" prefix; remaining part is the absolute path within the sysext.
		path := strings.TrimPrefix(line, "squashfs-root")
		if path == "" {
			continue
		}
		files[path] = true
	}
	return files, nil
}

// tdnfRepoqueryList queries the configured repos inside toolsChroot for the list of files that
// packageName would install. Returns the absolute file paths.
func tdnfRepoqueryList(
	toolsChroot *safechroot.Chroot,
	packageName string,
	pmHandler rpmPackageManagerHandler,
) ([]string, error) {
	args := []string{
		"-q",
		"--releasever=" + pmHandler.getReleaseVersion(),
		"--setopt=reposdir=" + rpmsMountParentDirInChroot,
		"repoquery",
		"--list",
		packageName,
	}

	stdout, _, err := shell.NewExecBuilder(pmHandler.getPackageManagerBinary(), args...).
		LogLevel(logrus.TraceLevel, logrus.DebugLevel).
		Chroot(toolsChroot.ChrootDir()).
		ExecuteCaptureOutput()
	if err != nil {
		return nil, fmt.Errorf("tdnf repoquery --list failed for (%s):\n%w", packageName, err)
	}

	var files []string
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// extractSysextRpmDb runs unsquashfs inside the tools chroot to extract only the var/lib/rpm
// subtree from the sysext, placing it at destChrootDir. Returns an error if the sysext does not
// contain an RPM DB (var/lib/rpm path not present).
func extractSysextRpmDb(toolsChroot *safechroot.Chroot, sysextChrootPath string, destChrootDir string) error {
	// Create the destination directory on the host (visible as destChrootDir inside the chroot).
	destHostDir := filepath.Join(toolsChroot.RootDir(), destChrootDir)
	if err := os.MkdirAll(destHostDir, 0o755); err != nil {
		return err
	}

	err := shell.NewExecBuilder("unsquashfs",
		"-dest", destChrootDir,
		sysextChrootPath,
		"var/lib/rpm",
	).
		LogLevel(logrus.TraceLevel, logrus.DebugLevel).
		Chroot(toolsChroot.ChrootDir()).
		Execute()
	if err != nil {
		return fmt.Errorf("unsquashfs extraction of RPM DB failed: %w", err)
	}

	// Verify the RPM DB was actually extracted.
	rpmDbHostPath := filepath.Join(destHostDir, "var", "lib", "rpm")
	if _, err := os.Stat(rpmDbHostPath); os.IsNotExist(err) {
		return fmt.Errorf("sysext does not contain an RPM DB (var/lib/rpm not found)")
	}
	return nil
}

// checkSysextRpmInventory implements Layer 2: check if any of the requested packages appear in
// the sysext's RPM DB. Finding a match is a hard error.
func checkSysextRpmInventory(
	toolsChroot *safechroot.Chroot,
	sysextName string,
	rpmDbChrootPath string,
	packages []string,
) error {
	for _, pkg := range packages {
		stdout, _, err := shell.NewExecBuilder("rpm",
			"--dbpath", rpmDbChrootPath,
			"-q", pkg,
			"--queryformat", "%{NAME}\n",
		).
			LogLevel(logrus.TraceLevel, logrus.DebugLevel).
			Chroot(toolsChroot.ChrootDir()).
			ExecuteCaptureOutput()
		if err != nil {
			// rpm -q exits non-zero when the package is not found; that is the expected (good) case.
			continue
		}
		if strings.TrimSpace(stdout) != "" {
			return fmt.Errorf(
				"package '%s' is already present in sysext '%s':\n"+
					"installing, updating, or removing a package that exists in a sysext will have no "+
					"visible effect at runtime (the sysext overlay takes precedence);\n"+
					"remove or replace the conflicting sysext before running image customizer",
				pkg, sysextName)
		}
	}
	return nil
}

// checkSysextDependencyIntegrity implements Layer 3: for each package being removed, check
// whether any package in the sysext's RPM DB declares it as a dependency. Such a removal would
// silently break the sysext at runtime.
func checkSysextDependencyIntegrity(
	toolsChroot *safechroot.Chroot,
	sysextName string,
	rpmDbChrootPath string,
	removePkgs []string,
) error {
	for _, pkg := range removePkgs {
		stdout, _, err := shell.NewExecBuilder("rpm",
			"--dbpath", rpmDbChrootPath,
			"-q", "--whatrequires", pkg,
			"--queryformat", "%{NAME}\n",
		).
			LogLevel(logrus.TraceLevel, logrus.DebugLevel).
			Chroot(toolsChroot.ChrootDir()).
			ExecuteCaptureOutput()
		if err != nil {
			// rpm -q --whatrequires exits non-zero when no dependents are found; that is the good case.
			continue
		}
		dependents := strings.TrimSpace(stdout)
		if dependents != "" {
			return fmt.Errorf(
				"package '%s' is required by sysext '%s' (packages: %s):\n"+
					"removing a package that a sysext depends on will break the sysext at runtime;\n"+
					"remove or replace the conflicting sysext before running image customizer",
				pkg, sysextName, strings.ReplaceAll(dependents, "\n", ", "))
		}
	}
	return nil
}
