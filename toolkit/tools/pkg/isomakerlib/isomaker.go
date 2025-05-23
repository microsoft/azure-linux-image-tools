// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package isomakerlib

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/configuration"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/isogenerator"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/jsonutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
)

const (
	isoRootArchDependentDirPath = "assets/isomaker/iso_root_arch-dependent_files"
	defaultImageNameBase        = "azure-linux"
	defaultOSFilesPath          = "isolinux"
	repoSnapshotFilePath        = "repo-snapshot-time.txt"
)

// IsoMaker builds ISO images and populates them with packages and files required by the installer.
type IsoMaker struct {
	enableBiosBoot     bool                 // Flag deciding whether to include BIOS bootloaders or not in the generated ISO image.
	enableRpmRepo      bool                 // Flag deciding whether to include the contents of the Rpm repo folder in the generated ISO image.
	unattendedInstall  bool                 // Flag deciding if the installer should run in unattended mode.
	config             configuration.Config // Configuration for the built ISO image and its installer.
	configSubDirNumber int                  // Current number for the subdirectories storing files mentioned in the config.
	baseDirPath        string               // Base directory for config's relative paths.
	buildDirPath       string               // Path to the temporary build directory.
	fetchedRepoDirPath string               // Path to the directory containing an RPM repository with all packages required by the ISO installer.
	initrdPath         string               // Path to ISO's initrd file.
	outputDirPath      string               // Path to the output ISO directory.
	releaseVersion     string               // Current Azure Linux release version.
	resourcesDirPath   string               // Path to the 'resources' directory.
	imageNameBase      string               // Base name of the ISO to generate (no path, and no file extension).
	imageNameTag       string               // Optional user-supplied tag appended to the generated ISO's name.
	repoSnapshotTime   string               // tdnf repo snapshot time
	osFilesPath        string

	isoMakerCleanUpTasks []func() error // List of clean-up tasks to perform at the end of the ISO generation process.
}

// NewIsoMaker returns a new ISO maker.
func NewIsoMaker(unattendedInstall bool, baseDirPath, buildDirPath, releaseVersion, resourcesDirPath, configFilePath, initrdPath, isoRepoDirPath, outputDir, imageNameTag, isoRepoSnapshotTime string) (isoMaker *IsoMaker, err error) {
	if baseDirPath == "" {
		baseDirPath = filepath.Dir(configFilePath)
	}

	imageNameBase := strings.TrimSuffix(filepath.Base(configFilePath), ".json")

	config, err := readConfigFile(configFilePath, baseDirPath)
	if err != nil {
		return nil, err
	}
	err = verifyConfig(config, unattendedInstall)
	if err != nil {
		return nil, err
	}

	isoMaker = &IsoMaker{
		enableBiosBoot:     true,
		enableRpmRepo:      true,
		unattendedInstall:  unattendedInstall,
		config:             config,
		baseDirPath:        baseDirPath,
		buildDirPath:       buildDirPath,
		initrdPath:         initrdPath,
		releaseVersion:     releaseVersion,
		resourcesDirPath:   resourcesDirPath,
		fetchedRepoDirPath: isoRepoDirPath,
		outputDirPath:      outputDir,
		imageNameBase:      imageNameBase,
		imageNameTag:       imageNameTag,
		osFilesPath:        defaultOSFilesPath,
		repoSnapshotTime:   isoRepoSnapshotTime,
	}

	return isoMaker, nil
}

// Make builds the ISO image to 'buildDirPath' with the packages included in the config JSON.
func (im *IsoMaker) Make() (err error) {
	defer func() {
		cleanupErr := im.isoMakerCleanUp()
		if cleanupErr != nil {
			if err != nil {
				err = fmt.Errorf("%w\nclean-up error: %w", err, cleanupErr)
			} else {
				err = fmt.Errorf("clean-up error: %w", cleanupErr)
			}
		}
	}()

	err = im.initializePaths()
	if err != nil {
		return err
	}

	err = im.prepareWorkDirectory()
	if err != nil {
		return err
	}

	err = im.createIsoRpmsRepo()
	if err != nil {
		return err
	}

	err = isogenerator.GenerateIso(isogenerator.IsoGenConfig{
		BuildDirPath:      im.buildDirPath,
		StagingDirPath:    im.buildDirPath,
		InitrdPath:        im.initrdPath,
		EnableBiosBoot:    im.enableBiosBoot,
		IsoOsFilesDirPath: im.osFilesPath,
		OutputFilePath:    im.buildIsoImageFilePath(),
	})
	if err != nil {
		return err
	}

	return nil
}

// createIsoRpmsRepo initializes the RPMs repo on the ISO image
// later accessed by the ISO installer.
func (im *IsoMaker) createIsoRpmsRepo() (err error) {
	if !im.enableRpmRepo {
		return
	}

	isoRpmsRepoDirPath := filepath.Join(im.buildDirPath, "RPMS")

	logger.Log.Debugf("Creating ISO RPMs repo under '%s'.", isoRpmsRepoDirPath)

	err = os.MkdirAll(isoRpmsRepoDirPath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to mkdir '%s'", isoRpmsRepoDirPath)
	}

	fetchedRepoDirContentsPath := filepath.Join(im.fetchedRepoDirPath, "*")
	err = recursiveCopyDereferencingLinks(fetchedRepoDirContentsPath, isoRpmsRepoDirPath)
	if err != nil {
		return err
	}

	return nil
}

// prepareWorkDirectory makes sure we start with a clean directory
// under "im.buildDirPath". The work directory will contain the contents of the ISO image.
func (im *IsoMaker) prepareWorkDirectory() (err error) {
	logger.Log.Infof("Building ISO under '%s'.", im.buildDirPath)

	exists, err := file.DirExists(im.buildDirPath)
	if err != nil {
		return fmt.Errorf("failed while checking if directory '%s' exists:\n%w", im.buildDirPath, err)
	}
	if exists {
		logger.Log.Warningf("Unexpected: temporary ISO build path '%s' exists. Removing", im.buildDirPath)
		err = os.RemoveAll(im.buildDirPath)
		if err != nil {
			return fmt.Errorf("failed while removing directory '%s':\n%w", im.buildDirPath, err)
		}
	}

	err = os.Mkdir(im.buildDirPath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed while creating directory '%s':\n%w", im.buildDirPath, err)
	}

	im.deferIsoMakerCleanUp(func() (removeErr error) {
		logger.Log.Debugf("Removing '%s'.", im.buildDirPath)
		removeErr = os.RemoveAll(im.buildDirPath)
		if removeErr != nil {
			removeErr = fmt.Errorf("failed to remove '%s':\n%w", im.buildDirPath, err)
		}
		return removeErr
	})

	err = im.copyStaticIsoRootFiles()
	if err != nil {
		return err
	}

	err = im.copyArchitectureDependentIsoRootFiles()
	if err != nil {
		return err
	}

	err = im.copyAndRenameConfigFiles()
	if err != nil {
		return err
	}

	return nil
}

// copyStaticIsoRootFiles copies architecture-independent files from the
// Azure Linux repo directories.
func (im *IsoMaker) copyStaticIsoRootFiles() (err error) {
	if im.resourcesDirPath == "" {
		return fmt.Errorf("missing required parameters. Must specify the resources directory")
	}

	staticIsoRootFilesPath := filepath.Join(im.resourcesDirPath, "assets/isomaker/iso_root_static_files/*")

	logger.Log.Debugf("Copying static ISO root files from '%s' to '%s'", staticIsoRootFilesPath, im.buildDirPath)

	err = recursiveCopyDereferencingLinks(staticIsoRootFilesPath, im.buildDirPath)
	if err != nil {
		return err
	}

	return nil
}

// copyArchitectureDependentIsoRootFiles copies the pre-built BIOS modules required
// to boot the ISO image.
func (im *IsoMaker) copyArchitectureDependentIsoRootFiles() error {
	// If the user does not want the generated ISO to have the BIOS bootloaders
	// (which are copied from the im.resourcesDirPath folder), the user can
	// either set im.resourcesDirPath to an empty string or enableBiosBoot to
	// false. Given that there is nothing else under the 'architecture
	// dependent` resource folder, if either of these two flags is set, we can
	// return immediately.
	// Note that setting resourcesDirPath to an empty string will affect other
	// functions that copy non-architecture dependent files. Setting
	// enableBiosBoot will not affect those on-architecture dependent files
	// though.
	if im.resourcesDirPath == "" && im.enableBiosBoot {
		return fmt.Errorf("missing required parameters. Must specify the resources directory if BIOS bootloaders are to be included")
	}

	if !im.enableBiosBoot {
		return nil
	}

	architectureDependentFilesDirectory := filepath.Join(im.resourcesDirPath, isoRootArchDependentDirPath, runtime.GOARCH, "*")

	logger.Log.Debugf("Copying architecture-dependent (%s) ISO root files from '%s'.", runtime.GOARCH, architectureDependentFilesDirectory)

	return recursiveCopyDereferencingLinks(architectureDependentFilesDirectory, im.buildDirPath)
}

// copyAndRenameConfigFiles takes care of copying the config JSON along with all the files
// required by the installed system.
func (im *IsoMaker) copyAndRenameConfigFiles() (err error) {
	const configDirName = "config"

	logger.Log.Debugf("Copying the config JSON and required files to the ISO's root.")

	configFilesAbsDirPath := filepath.Join(im.buildDirPath, configDirName)
	err = os.Mkdir(configFilesAbsDirPath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create ISO's config files directory under '%s':\n%w", configFilesAbsDirPath, err)
	}
	err = im.copyAndRenameAdditionalFiles(configFilesAbsDirPath)
	if err != nil {
		return err
	}
	err = im.copyAndRenamePackagesJSONs(configFilesAbsDirPath)
	if err != nil {
		return err
	}
	err = im.copyAndRenamePreInstallScripts(configFilesAbsDirPath)
	if err != nil {
		return err
	}
	err = im.copyAndRenamePostInstallScripts(configFilesAbsDirPath)
	if err != nil {
		return err
	}
	err = im.copyAndRenameFinalizeImageScripts(configFilesAbsDirPath)
	if err != nil {
		return err
	}
	err = im.copyAndRenameSSHPublicKeys(configFilesAbsDirPath)
	if err != nil {
		return err
	}
	err = im.saveConfigJSON(configFilesAbsDirPath)
	if err != nil {
		return err
	}

	// add snapshot file here
	err = im.addSnapshotTimeFile(configFilesAbsDirPath)
	if err != nil {
		return err
	}

	return nil
}

func (im *IsoMaker) addSnapshotTimeFile(configFilesAbsDirPath string) (err error) {
	if im.repoSnapshotTime != "" {
		logger.Log.Debugf("Adding snapshot time to file")
		err = file.WriteLines([]string{im.repoSnapshotTime}, path.Join(configFilesAbsDirPath, repoSnapshotFilePath))
		if err != nil {
			return
		}
	}
	return
}

// copyAndRenameAdditionalFiles will copy all additional files into an
// ISO directory to make them available to the installer.
// Each file gets placed in a separate directory to avoid potential name conflicts and
// the config gets updated with the new ISO paths.
func (im *IsoMaker) copyAndRenameAdditionalFiles(configFilesAbsDirPath string) (err error) {
	const additionalFilesSubDirName = "additionalfiles"

	for i := range im.config.SystemConfigs {
		systemConfig := &im.config.SystemConfigs[i]

		absAdditionalFiles := make(map[string]configuration.FileConfigList)
		for localAbsFilePath, installedSystemFileConfigs := range systemConfig.AdditionalFiles {
			isoRelativeFilePath, err := im.copyFileToConfigRoot(configFilesAbsDirPath, additionalFilesSubDirName, localAbsFilePath)
			if err != nil {
				return err
			}
			absAdditionalFiles[isoRelativeFilePath] = installedSystemFileConfigs
		}
		systemConfig.AdditionalFiles = absAdditionalFiles
	}

	return nil
}

// copyAndRenamePackagesJSONs will copy all package list JSONs into an
// ISO directory to make them available to the installer.
// Each file gets placed in a separate directory to avoid potential name conflicts and
// the config gets updated with the new ISO paths.
func (im *IsoMaker) copyAndRenamePackagesJSONs(configFilesAbsDirPath string) (err error) {
	const packagesSubDirName = "packages"

	for _, systemConfig := range im.config.SystemConfigs {
		for i, localPackagesAbsFilePath := range systemConfig.PackageLists {
			isoPackagesRelativeFilePath, err := im.copyFileToConfigRoot(configFilesAbsDirPath, packagesSubDirName, localPackagesAbsFilePath)
			if err != nil {
				return err
			}

			systemConfig.PackageLists[i] = isoPackagesRelativeFilePath
		}
	}

	return nil
}

// copyAndRenamePreInstallScripts will copy all pre-install scripts into an
// ISO directory to make them available to the installer.
// Each file gets placed in a separate directory to avoid potential name conflicts and
// the config gets updated with the new ISO paths.
func (im *IsoMaker) copyAndRenamePreInstallScripts(configFilesAbsDirPath string) (err error) {
	const preInstallScriptsSubDirName = "preinstallscripts"

	for _, systemConfig := range im.config.SystemConfigs {
		for i, localScriptAbsFilePath := range systemConfig.PreInstallScripts {
			isoScriptRelativeFilePath, err := im.copyFileToConfigRoot(configFilesAbsDirPath, preInstallScriptsSubDirName, localScriptAbsFilePath.Path)
			if err != nil {
				return err
			}

			systemConfig.PreInstallScripts[i].Path = isoScriptRelativeFilePath
		}
	}

	return nil
}

// copyAndRenamePostInstallScripts will copy all post-install scripts into an
// ISO directory to make them available to the installer.
// Each file gets placed in a separate directory to avoid potential name conflicts and
// the config gets updated with the new ISO paths.
func (im *IsoMaker) copyAndRenamePostInstallScripts(configFilesAbsDirPath string) (err error) {
	const postInstallScriptsSubDirName = "postinstallscripts"

	for _, systemConfig := range im.config.SystemConfigs {
		for i, localScriptAbsFilePath := range systemConfig.PostInstallScripts {
			isoScriptRelativeFilePath, err := im.copyFileToConfigRoot(configFilesAbsDirPath, postInstallScriptsSubDirName, localScriptAbsFilePath.Path)
			if err != nil {
				return err
			}

			systemConfig.PostInstallScripts[i].Path = isoScriptRelativeFilePath
		}
	}

	return nil
}

// copyAndRenameFinalizeImageScripts will copy all finalize-image scripts into an
// ISO directory to make them available to the installer.
// Each file gets placed in a separate directory to avoid potential name conflicts and
// the config gets updated with the new ISO paths.
func (im *IsoMaker) copyAndRenameFinalizeImageScripts(configFilesAbsDirPath string) (err error) {
	const finalizeImageScriptsSubDirName = "finalizeimagescripts"

	for _, systemConfig := range im.config.SystemConfigs {
		for i, localScriptAbsFilePath := range systemConfig.FinalizeImageScripts {
			isoScriptRelativeFilePath, err := im.copyFileToConfigRoot(configFilesAbsDirPath, finalizeImageScriptsSubDirName, localScriptAbsFilePath.Path)
			if err != nil {
				return err
			}

			systemConfig.FinalizeImageScripts[i].Path = isoScriptRelativeFilePath
		}
	}

	return nil
}

// copyAndRenameSSHPublicKeys will copy all SSH public keys into an
// ISO directory to make them available to the installer.
// Each file gets placed in a separate directory to avoid potential name conflicts and
// the config gets updated with the new ISO paths.
func (im *IsoMaker) copyAndRenameSSHPublicKeys(configFilesAbsDirPath string) (err error) {
	const sshPublicKeysSubDirName = "sshpublickeys"

	for _, systemConfig := range im.config.SystemConfigs {
		for _, user := range systemConfig.Users {
			for i, localSSHPublicKeyAbsPath := range user.SSHPubKeyPaths {
				isoSSHPublicKeyRelativeFilePath, err := im.copyFileToConfigRoot(configFilesAbsDirPath, sshPublicKeysSubDirName, localSSHPublicKeyAbsPath)
				if err != nil {
					return err
				}

				user.SSHPubKeyPaths[i] = isoSSHPublicKeyRelativeFilePath
			}
		}
	}

	return nil
}

// saveConfigJSON will save the modified config JSON into an
// ISO directory to make it available to the installer.
func (im *IsoMaker) saveConfigJSON(configFilesAbsDirPath string) (err error) {
	const (
		attendedInstallConfigFileName   = "attended_config.json"
		unattendedInstallConfigFileName = "unattended_config.json"
	)

	isoConfigFileAbsPath := filepath.Join(configFilesAbsDirPath, attendedInstallConfigFileName)
	if im.unattendedInstall {
		isoConfigFileAbsPath = filepath.Join(configFilesAbsDirPath, unattendedInstallConfigFileName)
	}

	err = jsonutils.WriteJSONFile(isoConfigFileAbsPath, &im.config)
	if err != nil {
		return fmt.Errorf("failed to save config JSON to '%s':\n%w", isoConfigFileAbsPath, err)
	}
	return nil
}

// copyFileToConfigRoot copies a single file to its own, numbered subdirectory to avoid name conflicts
// and returns the relative path to the file for the sake of config updates for the installer.
func (im *IsoMaker) copyFileToConfigRoot(configFilesAbsDirPath, configFilesSubDirName, localAbsFilePath string) (isoRelativeFilePath string, err error) {
	fileName := filepath.Base(localAbsFilePath)
	configFileSubDirRelativePath := fmt.Sprintf("%s/%d", configFilesSubDirName, im.configSubDirNumber)
	configFileSubDirAbsPath := filepath.Join(configFilesAbsDirPath, configFileSubDirRelativePath)

	err = os.MkdirAll(configFileSubDirAbsPath, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("failed to create ISO's config subdirectory '%s':\n%w", configFileSubDirAbsPath, err)
	}

	isoRelativeFilePath = filepath.Join(configFileSubDirRelativePath, fileName)
	isoAbsFilePath := filepath.Join(configFilesAbsDirPath, isoRelativeFilePath)

	logger.Log.Tracef("Copying file to ISO's config root '%s' from '%s'.", isoAbsFilePath, localAbsFilePath)

	err = file.Copy(localAbsFilePath, isoAbsFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to copy file to ISO's config root '%s' from '%s':\n%w", isoAbsFilePath, localAbsFilePath, err)
	}

	im.configSubDirNumber++

	return isoRelativeFilePath, nil
}

// initializePaths initializes absolute, global directory paths used by multiple other functions.
func (im *IsoMaker) initializePaths() (err error) {
	im.buildDirPath, err = filepath.Abs(im.buildDirPath)
	if err != nil {
		return fmt.Errorf("failed while retrieving absolute path from source root path: '%s':\n%w", im.buildDirPath, err)
	}

	return nil
}

// buildIsoImageFilePath gets the output ISO file path from the config JSON file name
// and the image build environment.
func (im *IsoMaker) buildIsoImageFilePath() string {
	isoImageFileNameSuffix := ""
	if im.releaseVersion != "" || im.imageNameTag != "" {
		isoImageFileNameSuffix = fmt.Sprintf("-%v%v", im.releaseVersion, im.imageNameTag)
	}
	isoImageFileName := fmt.Sprintf("%v%v.iso", im.imageNameBase, isoImageFileNameSuffix)

	return filepath.Join(im.outputDirPath, isoImageFileName)
}

// deferIsoMakerCleanUp accepts clean-up tasks to be ran when the entire
// build process has finished, NOT at the end of the current scope.
func (im *IsoMaker) deferIsoMakerCleanUp(cleanUpTask func() error) {
	im.isoMakerCleanUpTasks = append(im.isoMakerCleanUpTasks, cleanUpTask)
}

// isoMakerCleanUp runs all clean-up tasks scheduled through "deferIsoMakerCleanUp".
// Tasks are ran in reverse order to how they were scheduled.
func (im *IsoMaker) isoMakerCleanUp() (err error) {
	for i := len(im.isoMakerCleanUpTasks) - 1; i >= 0; i-- {
		cleanupErr := im.isoMakerCleanUpTasks[i]()
		if cleanupErr != nil {
			if err != nil {
				err = fmt.Errorf("%w\nclean-up error: %w", err, cleanupErr)
			} else {
				err = fmt.Errorf("clean-up error: %w", cleanupErr)
			}
		}
	}

	return err
}

func readConfigFile(configFilePath, baseDirPath string) (config configuration.Config, err error) {
	config, err = configuration.LoadWithAbsolutePaths(configFilePath, baseDirPath)
	if err != nil {
		return configuration.Config{}, fmt.Errorf("failed while reading config file from '%s' with base directory '%s':\n%w", configFilePath, baseDirPath, err)
	}
	return config, nil
}

func verifyConfig(config configuration.Config, unattendedInstall bool) error {

	// Set IsIsoInstall to true
	for id := range config.SystemConfigs {
		config.SystemConfigs[id].IsIsoInstall = true
	}

	if unattendedInstall && (len(config.SystemConfigs) > 1) && !config.DefaultSystemConfig.IsDefault {
		return fmt.Errorf("for unattended installation with more than one system configuration present you must select a default one with the [IsDefault] field")
	}
	return nil
}

// recursiveCopyDereferencingLinks simulates the behavior of "cp -r -L".
func recursiveCopyDereferencingLinks(source string, target string) (err error) {
	err = os.MkdirAll(target, os.ModePerm)
	if err != nil {
		return err
	}

	sourceToTarget := make(map[string]string)

	if filepath.Base(source) == "*" {
		filesToCopy, err := filepath.Glob(source)
		if err != nil {
			return err
		}
		for _, file := range filesToCopy {
			sourceToTarget[file] = target
		}
	} else {
		sourceToTarget[source] = target
	}

	for sourcePath, targetPath := range sourceToTarget {
		err = shell.ExecuteLive(false /*squashErrors*/, "cp", "-r", "-L", sourcePath, targetPath)
		if err != nil {
			return err
		}
	}

	return nil
}
