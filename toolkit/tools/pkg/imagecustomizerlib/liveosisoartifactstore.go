// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
)

const (
	// location on output iso where some of the input mic configuration will be
	// saved for future iso-to-iso customizations.
	savedConfigsDir = "azl-image-customizer"

	// file holding the iso kernel parameters from the input mic configuration
	// to be re-appended/merged with newer configures for future iso-to-iso
	// customizations.
	savedConfigsFileName     = "saved-configs.yaml"
	savedConfigsFileNamePath = "/" + savedConfigsDir + "/" + savedConfigsFileName
)

var (
	kernelVersionRegEx = regexp.MustCompile(`\b(\d+\.\d+\.\d+\.\d+-\d+\.(azl|cm)\d)(\b|kdump)`)
)

type IsoInfoStore struct {
	seLinuxMode              imagecustomizerapi.SELinuxMode
	kdumpBootFiles           *imagecustomizerapi.KdumpBootFilesType
	dracutPackageInfo        *PackageVersionInformation
	selinuxPolicyPackageInfo *PackageVersionInformation
}

type KernelBootFiles struct {
	vmlinuzPath     string
	initrdImagePath string
	otherFiles      []string
}

type KdumpBootFiles struct {
	vmlinuzPath     string
	initrdImagePath string
}

type IsoFilesStore struct {
	artifactsDir         string
	bootEfiPath          string
	grubEfiPath          string
	isoBootImagePath     string
	isoGrubCfgPath       string
	pxeGrubCfgPath       string
	savedConfigsFilePath string
	kernelBootFiles      map[string]*KernelBootFiles // kernel-version -> KernelBootFiles
	kdumpBootFiles       map[string]*KdumpBootFiles  // kernel-version -> kdumpBootFiles
	initrdImagePath      string                      // non-kernel specific
	squashfsImagePath    string
	additionalFiles      map[string]string // local-build-path -> iso-media-path
}

// `IsoArtifacts` holds the extracted/generated artifacts necessary to build
// a LiveOS ISO image.
type IsoArtifactsStore struct {
	info  *IsoInfoStore
	files *IsoFilesStore
}

func (b *IsoArtifactsStore) cleanUp() error {
	if b.files != nil {
		err := os.RemoveAll(b.files.artifactsDir)
		if err != nil {
			return fmt.Errorf("failed to clean-up (%s):\n%w", b.files.artifactsDir, err)
		}
	}
	return nil
}

func containsGrubNoPrefix(filePaths []string) (bool, error) {
	_, bootFilesConfig, err := getBootArchConfig()
	if err != nil {
		return false, err
	}
	for _, filePath := range filePaths {

		if filepath.Base(filePath) == bootFilesConfig.grubNoPrefixBinary {
			return true, nil
		}

	}
	return false, nil
}

func getSELinuxMode(imageChroot *safechroot.Chroot) (imagecustomizerapi.SELinuxMode, error) {
	bootCustomizer, err := NewBootCustomizer(imageChroot)
	if err != nil {
		return imagecustomizerapi.SELinuxModeDefault, err
	}

	imageSELinuxMode, err := bootCustomizer.GetSELinuxMode(imageChroot)
	if err != nil {
		return imagecustomizerapi.SELinuxModeDefault, fmt.Errorf("failed to get current SELinux mode:\n%w", err)
	}

	return imageSELinuxMode, nil
}

func getKernelVersions(filesStore *IsoFilesStore) []string {
	var kernelVersions []string
	for k := range filesStore.kernelBootFiles {
		kernelVersions = append(kernelVersions, k)
	}

	return kernelVersions
}

func storeIfKernelSpecificFile(filesStore *IsoFilesStore, targetPath string) bool {
	scheduleAdditionalFile := true

	baseFileName := filepath.Base(targetPath)

	matches := kernelVersionRegEx.FindStringSubmatch(baseFileName)
	if len(matches) <= 1 {
		return scheduleAdditionalFile
	}

	kernelVersion := matches[1]

	if strings.Contains(baseFileName, "kdump.img") {
		// Ensure we have an entry in the map for it
		kdumpBootFiles, exists := filesStore.kdumpBootFiles[kernelVersion]
		if !exists {
			kdumpBootFiles = &KdumpBootFiles{}
			filesStore.kdumpBootFiles[kernelVersion] = kdumpBootFiles
		}

		if strings.HasPrefix(baseFileName, initramfsPrefix) || strings.HasPrefix(baseFileName, initrdPrefix) {
			kdumpBootFiles.initrdImagePath = targetPath
			scheduleAdditionalFile = false
		}
	} else {
		// Ensure we have an entry in the map for it
		kernelBootFiles, exists := filesStore.kernelBootFiles[kernelVersion]
		if !exists {
			kernelBootFiles = &KernelBootFiles{}
			filesStore.kernelBootFiles[kernelVersion] = kernelBootFiles
		}

		if strings.HasPrefix(baseFileName, vmLinuzPrefix) {
			kernelBootFiles.vmlinuzPath = targetPath
			scheduleAdditionalFile = false
		} else if strings.HasPrefix(baseFileName, initramfsPrefix) || strings.HasPrefix(baseFileName, initrdPrefix) {
			kernelBootFiles.initrdImagePath = targetPath
			scheduleAdditionalFile = false
		} else {
			kernelBootFiles.otherFiles = append(kernelBootFiles.otherFiles, targetPath)
			scheduleAdditionalFile = false
		}
	}

	return scheduleAdditionalFile
}

func createIsoFilesStoreFromMountedImage(inputArtifactsStore *IsoArtifactsStore, imageRootDir string, storeDir string) (filesStore *IsoFilesStore, err error) {
	artifactsDir := filepath.Join(storeDir, "artifacts")

	filesStore = &IsoFilesStore{
		artifactsDir:         artifactsDir,
		savedConfigsFilePath: filepath.Join(artifactsDir, savedConfigsDir, savedConfigsFileName),
		additionalFiles:      make(map[string]string),
		kernelBootFiles:      make(map[string]*KernelBootFiles),
		kdumpBootFiles:       make(map[string]*KdumpBootFiles),
	}

	// the following files will be re-created - no need to copy them only to
	// have them overwritten.
	var exclusions []*regexp.Regexp
	//
	// On full disk images (generated by Mariner toolkit), there are two
	// grub.cfg files:
	// - <boot partition>/boot/grub2/grub.cfg:
	//   - mounted at /boot/efi/boot/grub2/grub.cfg.
	//   - empty except for redirection to the other grub.cfg.
	// - <rootfs partition>/boot/grub2/grub.cfg:
	//   - mounted at /boot/grub2/grub.cfg
	//   - has the actual grub configuration.
	//
	// When creating an iso image out of a full disk image, we do not need the
	// redirection mechanism, and hence we can do with only the full grub.cfg.
	//
	// To avoid confusion, we do not copy the redirection grub.cfg to the iso
	// media.
	//
	exclusions = append(exclusions, regexp.MustCompile(`/boot/efi/boot/grub2/grub\.cfg`))

	bootFolderFilePaths, err := file.EnumerateDirFiles(filepath.Join(imageRootDir, "/boot"))
	if err != nil {
		return nil, fmt.Errorf("failed to scan /boot folder:\n%w", err)
	}

	usingGrubNoPrefix, err := containsGrubNoPrefix(bootFolderFilePaths)
	if err != nil {
		return nil, err
	}

	for _, sourcePath := range bootFolderFilePaths {

		excluded := false
		for _, exclusion := range exclusions {
			match := exclusion.FindStringIndex(sourcePath)
			if match != nil {
				excluded = true
				break
			}
		}
		if excluded {
			logger.Log.Debugf("Not copying %s. File is either unnecessary or will be re-generated.", sourcePath)
			continue
		}

		relativeFilePath := strings.TrimPrefix(sourcePath, imageRootDir)
		targetPath := strings.Replace(sourcePath, imageRootDir, filesStore.artifactsDir, -1)

		// `scheduleAdditionalFile` indicates whether the file being processed
		// now should be captured in the general 'additional' files collection
		// or it has been captured in a data structure specific field
		// (like filesStore.isoGrubCfgPath) and it will be handled from there.
		// 'additional files' is a collection of all the files that need to be
		// included in the final output, but we do not have particular interest
		// in them (i.e. user files that we do not manipulate - but copy as-is).
		scheduleAdditionalFile := true

		_, bootFilesConfig, err := getBootArchConfig()
		if err != nil {
			return nil, err
		}
		osEspBootBinaryPath := bootFilesConfig.osEspBootBinaryPath
		osEspGrubBinaryPath := bootFilesConfig.osEspGrubBinaryPath
		osEspGrubNoPrefixBinaryPath := bootFilesConfig.osEspGrubNoPrefixBinaryPath

		switch relativeFilePath {
		case osEspBootBinaryPath:
			filesStore.bootEfiPath = targetPath
			scheduleAdditionalFile = false // No additional file scheduling

		case osEspGrubBinaryPath, osEspGrubNoPrefixBinaryPath:
			filesStore.grubEfiPath = targetPath
			scheduleAdditionalFile = false // No additional file scheduling

		case isoGrubCfgPath:
			if usingGrubNoPrefix {
				// When using the grubx64-noprefix.efi, the 'prefix' grub
				// variable is set to an empty string. When 'prefix' is an
				// empty string, and grubx64-noprefix.efi is run from an iso
				// media, the bootloader defaults to looking for grub.cfg at
				// <boot-media>/EFI/BOOT/grub.cfg.
				// So, below, we ensure that grub.cfg file will be placed where
				// grubx64-nopreifx.efi will be looking for it.
				//
				// Note that this grub.cfg is the only file that needs to be
				// copied to that EFI/BOOT location. The rest of the files (like
				// grubenv, etc) can be left under /boot as usual. This is
				// because grub.cfg still defines 'bootprefix' to be /boot.
				// So, once grubx64.efi loads EFI/BOOT/grub.cfg, it will set
				// bootprefix to the usual location boot/grub2 and will proceed
				// as usual from there.
				targetPath = filepath.Join(filesStore.artifactsDir, "EFI/BOOT", isoGrubCfg)
			}
			filesStore.isoGrubCfgPath = targetPath
			// We will place the pxe grub config next to the iso grub config.
			filesStore.pxeGrubCfgPath = filepath.Join(filepath.Dir(filesStore.isoGrubCfgPath), pxeGrubCfg)
			scheduleAdditionalFile = false
		case isoInitrdPath:
			filesStore.initrdImagePath = targetPath
			scheduleAdditionalFile = false
		default:
			scheduleAdditionalFile = storeIfKernelSpecificFile(filesStore, targetPath)
		}

		err = file.NewFileCopyBuilder(sourcePath, targetPath).
			SetNoDereference().
			Run()
		if err != nil {
			return nil, fmt.Errorf("failed to extract files from under the boot folder:\n%w", err)
		}

		if scheduleAdditionalFile {
			filesStore.additionalFiles[targetPath] = strings.TrimPrefix(targetPath, filesStore.artifactsDir)
		}
	}

	// Find the kdump kernel files matching the initramfs files found earlier.
	for kernelVersion, kdumpBootFiles := range filesStore.kdumpBootFiles {
		kernelBootFiles, exists := filesStore.kernelBootFiles[kernelVersion]
		if exists {
			kdumpBootFiles.vmlinuzPath = kernelBootFiles.vmlinuzPath
		}
	}

	if inputArtifactsStore != nil && inputArtifactsStore.files != nil {

		// Copy the saved config files from the input iso store
		inputSavedConfigsFilePath := inputArtifactsStore.files.savedConfigsFilePath
		exists, err := file.PathExists(inputSavedConfigsFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to check if the saved config file (%s) exists in the input iso.\n%w", inputSavedConfigsFilePath, err)
		}
		if exists {
			err = file.Copy(inputSavedConfigsFilePath, filesStore.savedConfigsFilePath)
			if err != nil {
				return nil, fmt.Errorf("failed to copy save configuration file from (%s) to (%s):\n%w",
					inputSavedConfigsFilePath, filesStore.savedConfigsFilePath, err)
			}
		}

		// If we started from an input iso (not an input vhd(x)/qcow), then there
		// might be additional files that are not defined in the current user
		// configuration. Below, we loop through the files we have captured so far
		// and append any file that was in the input iso and is not included
		// already. This also ensures that no file from the input iso overwrites
		// a newer version that has just been created.

		// Copy the additional files from the input iso store
		for inputSourceFile, inputTargetFile := range inputArtifactsStore.files.additionalFiles {
			found := false
			for _, targetFile := range filesStore.additionalFiles {
				if inputTargetFile == targetFile {
					found = true
					break
				}
			}

			if !found {
				// Copy the file from the 'input' store to the 'current' store.
				relativeFilePath := strings.TrimPrefix(inputSourceFile, inputArtifactsStore.files.artifactsDir)
				currentSourceFile := filepath.Join(filesStore.artifactsDir, relativeFilePath)
				err = file.Copy(inputSourceFile, currentSourceFile)
				if err != nil {
					return nil, fmt.Errorf("failed to copy (%s) to (%s):\n%w", inputSourceFile, currentSourceFile, err)
				}

				// Update map
				filesStore.additionalFiles[currentSourceFile] = inputTargetFile
			}
		}
	}

	_, bootFilesConfig, err := getBootArchConfig()
	if err != nil {
		return nil, err
	}
	if filesStore.bootEfiPath == "" {
		return nil, fmt.Errorf("failed to find the boot efi file (%s):\n"+
			"this file is provided by the (shim) package",
			bootFilesConfig.bootBinary)
	}

	if filesStore.grubEfiPath == "" {
		return nil, fmt.Errorf("failed to find the grub efi file (%s or %s):\n"+
			"this file is provided by either the (grub2-efi-binary) or the (grub2-efi-binary-noprefix) package",
			bootFilesConfig.grubBinary, bootFilesConfig.grubNoPrefixBinary)
	}

	return filesStore, nil
}

func createIsoInfoStoreFromMountedImage(imageRootDir string) (infoStore *IsoInfoStore, err error) {
	infoStore = &IsoInfoStore{}

	chroot := safechroot.NewChroot(imageRootDir, true /*isExistingDir*/)
	if chroot == nil {
		return nil, fmt.Errorf("failed to create a new chroot object for (%s)", imageRootDir)
	}
	defer chroot.Close(true /*leaveOnDisk*/)

	err = chroot.Initialize("", nil, nil, true /*includeDefaultMounts*/)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize chroot object for (%s):\n%w", imageRootDir, err)
	}

	imageSELinuxMode, err := getSELinuxMode(chroot)
	if err != nil {
		return nil, fmt.Errorf("failed to determine SELinux mode for (%s):\n%w", imageRootDir, err)
	}
	infoStore.seLinuxMode = imageSELinuxMode

	infoStore.dracutPackageInfo, err = getPackageInformation(chroot, "dracut")
	if err != nil {
		return nil, fmt.Errorf("failed to determine package information for dracut under (%s):\n%w", imageRootDir, err)
	}

	// Note the MIC allows the user to install other selinux policy packages.
	// So, the absence of selinux-policy does not mean that there are no selinux
	// policy packages.
	distroHandler, err := getDistroHandlerFromChroot(chroot)
	if err != nil {
		return nil, fmt.Errorf("failed to determine distro from chroot:\n%w", err)
	}

	if distroHandler.isPackageInstalled(chroot, "selinux-policy") {
		infoStore.selinuxPolicyPackageInfo, err = getPackageInformation(chroot, "selinux-policy")
		if err != nil {
			return nil, fmt.Errorf("failed to determine package information for selinux-policy under (%s):\n%w", imageRootDir, err)
		}
	}

	return infoStore, nil
}

func createIsoFilesStoreFromIsoImage(isoImageFile, storeDir string) (filesStore *IsoFilesStore, err error) {
	artifactsDir := filepath.Join(storeDir, "artifacts")

	filesStore = &IsoFilesStore{
		artifactsDir:         artifactsDir,
		savedConfigsFilePath: filepath.Join(artifactsDir, savedConfigsDir, savedConfigsFileName),
	}

	err = extractIsoImageContents(storeDir, isoImageFile, filesStore.artifactsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to extract iso contents from input iso file (%s):\n%w", isoImageFile, err)
	}

	isoFiles, err := file.EnumerateDirFiles(artifactsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate expanded iso files under %s:\n%w", artifactsDir, err)
	}

	filesStore.additionalFiles = make(map[string]string)
	filesStore.kernelBootFiles = make(map[string]*KernelBootFiles)
	filesStore.kdumpBootFiles = make(map[string]*KdumpBootFiles)

	_, bootFilesConfig, err := getBootArchConfig()
	if err != nil {
		return nil, err
	}
	isoBootBinaryPath := bootFilesConfig.isoBootBinaryPath
	isoGrubBinaryPath := bootFilesConfig.isoGrubBinaryPath

	for _, isoFile := range isoFiles {
		relativeFilePath := strings.TrimPrefix(isoFile, artifactsDir)

		scheduleAdditionalFile := true

		switch relativeFilePath {
		case isoBootBinaryPath:
			filesStore.bootEfiPath = isoFile
			scheduleAdditionalFile = false
		case isoGrubBinaryPath:
			// Note that grubx64NoPrefixBinary is not expected to be present on
			// an existing iso - and hence we do not look for it here.
			// grubx64NoPrefixBinary may exist only on a vhdx/qcow when the
			// grub-noprefix package is installed. When such images are
			// converted to an iso, we rename the grub binary to its regular
			// name (grubx64.efi).
			filesStore.grubEfiPath = isoFile
			scheduleAdditionalFile = false
		case isoGrubCfgPath:
			filesStore.isoGrubCfgPath = isoFile
			// We will place the pxe grub config next to the iso grub config.
			filesStore.pxeGrubCfgPath = filepath.Join(filepath.Dir(filesStore.isoGrubCfgPath), pxeGrubCfg)
			scheduleAdditionalFile = false
		case liveOSImagePath:
			filesStore.squashfsImagePath = isoFile
			// the squashfs image file is added to the additional file list
			// by a different part of the code
			scheduleAdditionalFile = false
		case isoInitrdPath:
			filesStore.initrdImagePath = isoFile
			scheduleAdditionalFile = false
		case savedConfigsFileNamePath:
			filesStore.savedConfigsFilePath = isoFile
			scheduleAdditionalFile = false
		case isoBootImagePath:
			filesStore.isoBootImagePath = isoFile
			scheduleAdditionalFile = false
		default:
			scheduleAdditionalFile = storeIfKernelSpecificFile(filesStore, isoFile)
		}

		if scheduleAdditionalFile {
			filesStore.additionalFiles[isoFile] = strings.TrimPrefix(isoFile, artifactsDir)
		}
	}

	return filesStore, nil
}

func createIsoInfoStoreFromIsoImage(savedConfigFile string) (infoStore *IsoInfoStore, err error) {
	savedConfigs, err := loadSavedConfigs(savedConfigFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load saved configurations (%s):\n%w", savedConfigFile, err)
	}

	infoStore = &IsoInfoStore{}

	// Need to populate the dracut package information from the saved copy
	// since we will not expand the rootfs and inspect its contents to get
	// such information.
	infoStore = &IsoInfoStore{
		kdumpBootFiles:           savedConfigs.LiveOS.KdumpBootFiles,
		dracutPackageInfo:        savedConfigs.OS.DracutPackageInfo,
		selinuxPolicyPackageInfo: savedConfigs.OS.SELinuxPolicyPackageInfo,
	}

	return infoStore, nil
}

func createIsoArtifactStoreFromMountedImage(inputArtifactsStore *IsoArtifactsStore, imageRootDir string, storeDir string) (artifactStore *IsoArtifactsStore, err error) {
	err = os.MkdirAll(storeDir, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to create folder %s:\n%w", storeDir, err)
	}

	artifactStore = &IsoArtifactsStore{}

	filesStore, err := createIsoFilesStoreFromMountedImage(inputArtifactsStore, imageRootDir, storeDir)
	if err != nil {
		return nil, err
	}
	artifactStore.files = filesStore

	infoStore, err := createIsoInfoStoreFromMountedImage(imageRootDir)
	if err != nil {
		return nil, err
	}
	artifactStore.info = infoStore

	return artifactStore, nil
}

func createIsoArtifactStoreFromIsoImage(isoImageFile, storeDir string) (artifactStore *IsoArtifactsStore, err error) {
	logger.Log.Debugf("Creating ISO store (%s) from (%s)", storeDir, isoImageFile)

	err = os.MkdirAll(storeDir, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to create folder %s:\n%w", storeDir, err)
	}

	artifactStore = &IsoArtifactsStore{}

	filesStore, err := createIsoFilesStoreFromIsoImage(isoImageFile, storeDir)
	if err != nil {
		return nil, err
	}
	artifactStore.files = filesStore

	infoStore, err := createIsoInfoStoreFromIsoImage(filesStore.savedConfigsFilePath)
	if err != nil {
		return nil, err
	}
	artifactStore.info = infoStore

	return artifactStore, nil
}

func fileExistsToString(filePath string) string {
	exists, err := file.PathExists(filePath)
	if err != nil {
		return fmt.Sprintf("%s (failed to check file):%s", filePath, err.Error())
	}
	if exists {
		return " e " + filePath
	}
	return "!e " + filePath
}

func dumpKernelBootFiles(kernelBootFiles *KernelBootFiles) {
	if kernelBootFiles == nil {
		logger.Log.Debugf("-- -- not defined")
		return
	}

	logger.Log.Debugf("-- -- vmlinuzPath           = %s", fileExistsToString(kernelBootFiles.vmlinuzPath))
	logger.Log.Debugf("-- -- initrdImagePath       = %s", fileExistsToString(kernelBootFiles.initrdImagePath))
	for _, otherFile := range kernelBootFiles.otherFiles {
		logger.Log.Debugf("-- -- otherFile             = %s", fileExistsToString(otherFile))
	}
}

func dumpKdumpBootFiles(kdumpBootFiles *KdumpBootFiles) {
	if kdumpBootFiles == nil {
		logger.Log.Debugf("-- -- not defined")
		return
	}

	logger.Log.Debugf("-- -- vmlinuzPath           = %s", fileExistsToString(kdumpBootFiles.vmlinuzPath))
	logger.Log.Debugf("-- -- initrdImagePath       = %s", fileExistsToString(kdumpBootFiles.initrdImagePath))
}

func dumpFilesStore(filesStore *IsoFilesStore) {
	logger.Log.Debugf("Files Store")
	if filesStore == nil {
		logger.Log.Debugf("-- not defined")
		return
	}
	logger.Log.Debugf("-- artifactsDir             = %s", fileExistsToString(filesStore.artifactsDir))
	logger.Log.Debugf("-- bootEfiPath              = %s", fileExistsToString(filesStore.bootEfiPath))
	logger.Log.Debugf("-- grubEfiPath              = %s", fileExistsToString(filesStore.grubEfiPath))
	logger.Log.Debugf("-- isoBootImagePath         = %s", fileExistsToString(filesStore.isoBootImagePath))
	logger.Log.Debugf("-- isoGrubCfgPath           = %s", fileExistsToString(filesStore.isoGrubCfgPath))
	logger.Log.Debugf("-- pxeGrubCfgPath           = %s", fileExistsToString(filesStore.pxeGrubCfgPath))
	logger.Log.Debugf("-- savedConfigsFilePath     = %s", fileExistsToString(filesStore.savedConfigsFilePath))
	logger.Log.Debugf("-- kernel file groups")
	for key, value := range filesStore.kernelBootFiles {
		logger.Log.Debugf("-- - version               = %s", key)
		dumpKernelBootFiles(value)
	}
	logger.Log.Debugf("-- kdump file groups")
	for key, value := range filesStore.kdumpBootFiles {
		logger.Log.Debugf("-- - version               = %s", key)
		dumpKdumpBootFiles(value)
	}
	logger.Log.Debugf("-- initrdImagePath          = %s", fileExistsToString(filesStore.initrdImagePath))
	logger.Log.Debugf("-- squashfsImagePath        = %s", fileExistsToString(filesStore.squashfsImagePath))
	logger.Log.Debugf("-- additionalFiles          =")
	for key, value := range filesStore.additionalFiles {
		logger.Log.Debugf("-- -- localPath: %s, isoPath: %s\n", fileExistsToString(key), value)
	}
}

func dumpInfoStore(infoStore *IsoInfoStore) {
	logger.Log.Debugf("Info Store")
	if infoStore == nil {
		logger.Log.Debugf("-- not defined")
		return
	}
	if infoStore.kdumpBootFiles != nil {
		logger.Log.Debugf("-- kdumpBootFiles       = %s", *infoStore.kdumpBootFiles)
	} else {
		logger.Log.Debugf("-- kdumpBootFiles       = not defined")
	}
	logger.Log.Debugf("-- seLinuxMode          = %s", infoStore.seLinuxMode)
	if infoStore.dracutPackageInfo != nil {
		logger.Log.Debugf("-- dracut package info  = %s", infoStore.dracutPackageInfo.getFullVersionString())
	} else {
		logger.Log.Debugf("-- dracut package info  = unavailable")
	}
	if infoStore.selinuxPolicyPackageInfo != nil {
		logger.Log.Debugf("-- selinux package info = %s", infoStore.selinuxPolicyPackageInfo.getFullVersionString())
	} else {
		logger.Log.Debugf("-- selinux package info = unavailable")
	}
}

func dumpArtifactsStore(artifactStore *IsoArtifactsStore, title string) {
	logger.Log.Debugf("Artifacts Store - %s", title)
	if artifactStore == nil {
		logger.Log.Debugf("-- not defined")
		return
	}
	dumpFilesStore(artifactStore.files)
	dumpInfoStore(artifactStore.info)
}
