// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/packagerepo/repomanager/rpmrepomanager"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safemount"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sys/unix"
	"gopkg.in/ini.v1"
)

var (
	// RPM source mount errors
	ErrRpmSourceFileTypeDetection = NewImageCustomizerError("RpmSources:FileTypeDetection", "failed to get type of RPM source")
	ErrRpmSourceTypeUnknown       = NewImageCustomizerError("RpmSources:TypeUnknown", "unknown RPM source type")
)

const (
	rpmsMountParentDirInChroot = "/_localrpms"
)

// Used to manage (including cleanup) the mounts required by package installation/update.
type rpmSourcesMounts struct {
	rpmsMountParentDir        string
	rpmsMountParentDirCreated bool
	mounts                    []*safemount.Mount
	allReposConfigFilePath    string
}

func mountRpmSources(ctx context.Context, buildDir string, imageChroot *safechroot.Chroot, rpmsSources []string,
	useBaseImageRpmRepos bool,
) (*rpmSourcesMounts, error) {
	var err error

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "mount_rpm_sources")
	span.SetAttributes(
		attribute.Int("rpm_sources_count", len(rpmsSources)),
		attribute.Bool("use_base_image_rpm_repos", useBaseImageRpmRepos),
	)
	defer span.End()

	var mounts rpmSourcesMounts
	err = mounts.mountRpmSourcesHelper(buildDir, imageChroot, rpmsSources, useBaseImageRpmRepos)
	if err != nil {
		cleanupErr := mounts.close()
		if cleanupErr != nil {
			logger.Log.Warnf("rpm sources mount cleanup failed: %s", cleanupErr)
		}
		return nil, err
	}

	return &mounts, nil
}

func (m *rpmSourcesMounts) mountRpmSourcesHelper(buildDir string, imageChroot *safechroot.Chroot, rpmsSources []string,
	useBaseImageRpmRepos bool,
) error {
	var err error

	m.rpmsMountParentDir = path.Join(imageChroot.RootDir(), rpmsMountParentDirInChroot)

	// Create temporary directory for RPM sources to be mounted (and fail if it already exists).
	err = os.Mkdir(m.rpmsMountParentDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create source rpms directory (%s):\n%w", m.rpmsMountParentDir, err)
	}

	m.rpmsMountParentDirCreated = true

	// Unfortunatley, tdnf doesn't support the repository priority field.
	// So, to ensure repos are used in the correct order, create a single config file containing all the repos, specified
	// in the order of highest priority to lowest priority.
	allReposConfig := ini.Empty()

	// Include base image's RPM sources.
	if useBaseImageRpmRepos {
		reposPath := filepath.Join(imageChroot.RootDir(), "/etc/yum.repos.d")
		entries, err := os.ReadDir(reposPath)
		if err != nil {
			return fmt.Errorf("failed to read base image's repos directory:\n%w", err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			name := entry.Name()
			if !strings.HasSuffix(name, ".repo") {
				continue
			}

			repoFilePath := filepath.Join(reposPath, name)
			err = m.createRepoFromRepoConfig(repoFilePath, false, allReposConfig, imageChroot)
			if err != nil {
				return fmt.Errorf("failed to add base image's repo (%s):\n%w", name, err)
			}
		}
	}

	// Mount the RPM sources.
	for _, rpmSource := range rpmsSources {
		fileType, err := getRpmSourceFileType(rpmSource)
		if err != nil {
			return fmt.Errorf("failed to get RPM source file type (%s):\n%w", rpmSource, err)
		}

		switch fileType {
		case "dir":
			err = m.createRepoFromDirectory(rpmSource, allReposConfig, imageChroot)

		case "repo":
			err = m.createRepoFromRepoConfig(rpmSource, true, allReposConfig, imageChroot)
		}
		if err != nil {
			return err
		}
	}

	// Create all-repos config file.
	m.allReposConfigFilePath = filepath.Join(imageChroot.RootDir(), rpmsMountParentDirInChroot, "allrepos.repo")
	logger.Log.Debugf("Writing allrepos.repo (%s)", m.allReposConfigFilePath)

	err = allReposConfig.SaveTo(m.allReposConfigFilePath)
	if err != nil {
		return fmt.Errorf("failed to save all-repos config file (%s):\n%w", m.allReposConfigFilePath, err)
	}

	if logger.Log.IsLevelEnabled(logrus.TraceLevel) {
		allReposConfigString, err := os.ReadFile(m.allReposConfigFilePath)
		if err == nil {
			logger.Log.Tracef("allrepos.repo:\n%s", allReposConfigString)
		}
	}

	return nil
}

func (m *rpmSourcesMounts) createRepoFromDirectory(rpmSource string, allReposConfig *ini.File,
	imageChroot *safechroot.Chroot,
) error {
	// Turn directory into an RPM repo.
	err := rpmrepomanager.CreateOrUpdateRepo(rpmSource)
	if err != nil {
		return fmt.Errorf("failed create RPMs repo from directory (%s):\n%w", rpmSource, err)
	}

	rpmSourceName := path.Base(rpmSource)

	// Mount the directory.
	mountTargetDirectoryInChroot, err := m.mountRpmsSource("baseurl", rpmSourceName, rpmSource, imageChroot)
	if err != nil {
		return err
	}

	// Add local repo config.
	err = appendLocalRepo(allReposConfig, mountTargetDirectoryInChroot, rpmSource)
	if err != nil {
		return fmt.Errorf("failed to append local repo config:\n%w", err)
	}

	return nil
}

func (m *rpmSourcesMounts) createRepoFromRepoConfig(rpmSource string, isHostConfig bool, allReposConfig *ini.File,
	imageChroot *safechroot.Chroot,
) error {
	// Parse the repo config file.
	reposConfig, err := ini.Load(rpmSource)
	if err != nil {
		return fmt.Errorf("failed load repo config file (%s):\n%w", rpmSource, err)
	}

	// Iterate through the list of repos.
	for _, repoConfig := range reposConfig.Sections() {
		if repoConfig.Name() == ini.DefaultSection {
			if len(repoConfig.Keys()) > 0 {
				return fmt.Errorf("rpm repo config files must not contain a default section (%s)", rpmSource)
			}

			continue
		}

		if isHostConfig {
			_, err = repoConfig.GetKey("baseurl")
			if err != nil {
				return fmt.Errorf("invalid repo config (%s):\n%w", rpmSource, err)
			}

			gpgCheckKey, _ := repoConfig.GetKey("gpgcheck")
			repoGpgCheckKey, _ := repoConfig.GetKey("repo_gpgcheck")
			switch {
			case gpgCheckKey != nil && gpgCheckKey.String() != "1":
				logger.Log.Infof("GPG signature checking disabled for RPM repo (%s)", repoConfig.Name())

			case repoGpgCheckKey != nil && repoGpgCheckKey.String() != "1":
				logger.Log.Infof("GPG signature checking disabled for RPM repo metadata (%s)", repoConfig.Name())
			}

			for _, field := range []string{"baseurl", "gpgkey"} {
				fieldKey, err := repoConfig.GetKey(field)
				if err != nil {
					// Field doesn't exist.
					continue
				}

				fieldValue := fieldKey.String()
				values := strings.Fields(fieldValue)

				newValues := []string(nil)
				for _, value := range values {
					// Check if the value points to a local file/directory.
					filePath, hasFilePrefix := strings.CutPrefix(value, "file://")
					if hasFilePrefix {
						// Bind mount the file/directory in the chroot.
						rpmSourceName := path.Base(value)
						mountTargetDirectoryInChroot, err := m.mountRpmsSource(field, rpmSourceName, filePath,
							imageChroot)
						if err != nil {
							return fmt.Errorf("failed mount repo config local file/directory (%s):\n%w", filePath, err)
						}

						// Change the value to point to the bind mount file/directory.
						newValue := fmt.Sprintf("file://%s", mountTargetDirectoryInChroot)
						newValues = append(newValues, newValue)
					} else {
						newValues = append(newValues, value)
					}
				}

				newFieldValue := strings.Join(newValues, " ")
				repoConfig.Key(field).SetValue(newFieldValue)
			}
		}

		// Copy over the repo details to the all-repos config.
		err := appendIniSection(allReposConfig, repoConfig)
		if err != nil {
			return fmt.Errorf("failed to append repo config (%s):\n%w", rpmSource, err)
		}
	}

	return nil
}

func (m *rpmSourcesMounts) mountRpmsSource(fieldName string, sourceName string, sourcePath string,
	imageChroot *safechroot.Chroot,
) (string, error) {
	i := len(m.mounts)
	targetName := fmt.Sprintf("%02d-%s-%s", i, fieldName, sourceName)
	mountTargetDirectoryInChroot := path.Join(rpmsMountParentDirInChroot, targetName)
	mountTargetDirectory := path.Join(imageChroot.RootDir(), mountTargetDirectoryInChroot)

	// Create a read-only bind mount.
	mount, err := safemount.NewMount(sourcePath, mountTargetDirectory, "", unix.MS_BIND|unix.MS_RDONLY, "", true)
	if err != nil {
		return "", fmt.Errorf("failed to mount RPM source from (%s) to (%s):\n%w", sourcePath, mountTargetDirectory, err)
	}

	m.mounts = append(m.mounts, mount)
	return mountTargetDirectoryInChroot, nil
}

func (m *rpmSourcesMounts) close() error {
	var err error
	var errs []error

	// Delete allrepos.repo file (if it exists).
	err = os.RemoveAll(m.allReposConfigFilePath)
	if err != nil {
		errs = append(errs, err)
	}

	// Unmount rpm source directories.
	for _, mount := range m.mounts {
		err = mount.CleanClose()
		if err != nil {
			errs = append(errs, err)
			continue
		}
	}

	// Join all the errors together.
	if len(errs) > 0 {
		err = errors.Join(errs...)
		err = fmt.Errorf("failed to cleanup RPM sources mounts:\n%w", err)
		return err
	}

	// Delete the temporary directory.
	if m.rpmsMountParentDirCreated {
		// Note: Do not use `RemoveAll` here in case there are any leftover mounts that failed to unmount.
		err = os.Remove(m.rpmsMountParentDir)
		if err != nil {
			return fmt.Errorf("failed to delete source rpms directory (%s):\n%w", m.rpmsMountParentDir, err)
		}

		m.rpmsMountParentDirCreated = false
	}

	return nil
}

func ValidateRpmSources(rpmsSources []string) error {
	for _, rpmSource := range rpmsSources {
		_, err := getRpmSourceFileType(rpmSource)
		if err != nil {
			return err
		}
	}

	return nil
}

func getRpmSourceFileType(rpmSourcePath string) (string, error) {
	// First, check if path points to a directory.
	isDir, err := file.IsDir(rpmSourcePath)
	if err != nil {
		return "", fmt.Errorf("%w (path='%s'):\n%w", ErrRpmSourceFileTypeDetection, rpmSourcePath, err)
	}

	if isDir {
		return "dir", nil
	}

	filename := filepath.Base(rpmSourcePath)
	dotIndex := strings.LastIndex(filename, ".")
	fileExt := ""
	if dotIndex >= 0 {
		fileExt = filename[dotIndex:]
	}

	switch fileExt {
	case ".repo":
		return "repo", nil

	default:
		return "", fmt.Errorf("%w (path='%s'):\nmust be a .repo file or a directory", ErrRpmSourceTypeUnknown, rpmSourcePath)
	}
}

// Add a local directory containing RPMs to the allrepos.repo file.
func appendLocalRepo(iniFile *ini.File, mountTargetDirectoryInChroot string, rpmSource string) error {
	repoName := filepath.Base(mountTargetDirectoryInChroot)
	iniSection, err := iniFile.NewSection(repoName)
	if err != nil {
		return err
	}

	_, err = iniSection.NewKey("name", repoName)
	if err != nil {
		return err
	}

	baseurl := fmt.Sprintf("file://%s", mountTargetDirectoryInChroot)

	_, err = iniSection.NewKey("baseurl", baseurl)
	if err != nil {
		return err
	}

	_, err = iniSection.NewKey("enabled", "1")
	if err != nil {
		return err
	}

	// Disable GPG checks for local directories.
	// There is no API to specify the GPG public key to use to verify the packages in the local directories.
	// Also, local directories are likely to contain a user's custom built packages, which are very unlikely to be
	// signed. If a user does sign their own packages, then they can pass in a .repo file instead and set the 'gpgkey'
	// field within the .repo file.
	_, err = iniSection.NewKey("gpgcheck", "0")
	if err != nil {
		return err
	}

	_, err = iniSection.NewKey("repo_gpgcheck", "0")
	if err != nil {
		return err
	}

	logger.Log.Infof("GPG signature checking disabled for RPM repo (%s)", rpmSource)
	return nil
}

// appendIniSection copies an ini section to the end of an ini file.
func appendIniSection(iniFile *ini.File, iniSection *ini.Section) error {
	newSection, err := iniFile.NewSection(iniSection.Name())
	if err != nil {
		return err
	}

	for _, key := range iniSection.Keys() {
		_, err := newSection.NewKey(key.Name(), key.Value())
		if err != nil {
			return err
		}
	}

	return nil
}
