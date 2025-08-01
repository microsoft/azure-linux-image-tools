// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package safechroot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"time"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/buildpipeline"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/processes"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/retry"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/systemdependency"

	"github.com/moby/sys/mountinfo"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// BindMountPointFlags is a set of flags to do a bind mount.
const BindMountPointFlags = unix.MS_BIND | unix.MS_MGC_VAL

// FileToCopy represents a file to copy into a chroot using AddFiles. Dest is relative to the chroot directory.
type FileToCopy struct {
	// The source file path.
	// Mutually exclusive with 'Content'.
	Src string
	// The contents of the file to write.
	// Mutually exclusive with 'Src'.
	Content *string
	// The destination path to write/copy the file to.
	Dest        string
	Permissions *os.FileMode
	// Set to true to copy symlinks as symlinks.
	NoDereference bool
}

// DirToCopy represents a directory to copy into a chroot using AddDirs. Dest is relative to the chroot directory.
type DirToCopy struct {
	Src                  string
	Dest                 string
	NewDirPermissions    os.FileMode
	ChildFilePermissions os.FileMode
	MergedDirPermissions *os.FileMode
}

// MountPoint represents a system mount point used by a Chroot.
// It is guaranteed to be unmounted on application exit even on a SIGTERM so long as registerSIGTERMCleanup is invoked.
// The fields of MountPoint mirror those of the `mount` syscall.
type MountPoint struct {
	source string
	target string
	fstype string
	flags  uintptr
	data   string

	isMounted           bool
	mountBeforeDefaults bool
}

// Chroot represents a Chroot environment with automatic synchronization protections
// and guaranteed cleanup code even on SIGTERM so long as registerSIGTERMCleanup is invoked.
type Chroot struct {
	rootDir     string
	mountPoints []*MountPoint

	isExistingDir        bool
	includeDefaultMounts bool
}

// inChrootMutex guards against multiple Chroots entering their respective Chroots
// and running commands. Only a single Chroot can be active at a given time.
//
// activeChrootsMutex guards activeChroots reads and writes.
//
// activeChroots is slice of Initialized Chroots that should be cleaned up iff
// registerSIGTERMCleanup has been invoked. Use a slice instead of a map
// to ensure chroots can be cleaned up in LIFO order incase any are interdependent.
// Note:
//   - Docker based build doesn't need to maintain activeChroots because chroot come from
//     a pre-existing pool of chroots
//     (as opposed to regular build which create a new chroot each time a spec is built)
var (
	inChrootMutex      sync.Mutex
	activeChrootsMutex sync.Mutex
	activeChroots      []*Chroot
)

var defaultChrootEnv = []string{
	"USER=root",
	"HOME=/root",
	fmt.Sprintf("SHELL=%s", os.Getenv("SHELL")),
	fmt.Sprintf("TERM=%s", os.Getenv("TERM")),
	"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
}

const (
	unmountTypeLazy   = true
	unmountTypeNormal = !unmountTypeLazy
)

// init will always be called if this package is loaded
func init() {
	registerSIGTERMCleanup()
	logrus.RegisterExitHandler(cleanupAllChroots)
}

// NewMountPoint creates a new MountPoint struct to be created by a Chroot
func NewMountPoint(source, target, fstype string, flags uintptr, data string) (mountPoint *MountPoint) {
	return &MountPoint{
		source: source,
		target: target,
		fstype: fstype,
		flags:  flags,
		data:   data,
	}
}

// NewPreDefaultsMountPoint creates a new MountPoint struct to be created by a Chroot but before the default mount points.
func NewPreDefaultsMountPoint(source, target, fstype string, flags uintptr, data string) (mountPoint *MountPoint) {
	return &MountPoint{
		source:              source,
		target:              target,
		fstype:              fstype,
		flags:               flags,
		data:                data,
		mountBeforeDefaults: true,
	}
}

// NewOverlayMountPoint creates a new MountPoint struct and extra directories slice configured for a given overlay
func NewOverlayMountPoint(chrootDir, source, target, lowerDir, upperDir, workDir string) (mountPoint *MountPoint, extaDirsNeeds []string) {
	const (
		overlayFlags  = 0
		overlayFsType = "overlay"
	)

	upperDirInChroot := filepath.Join(chrootDir, upperDir)
	workDirInChroot := filepath.Join(chrootDir, workDir)

	overlayData := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lowerDir, upperDirInChroot, workDirInChroot)

	extaDirsNeeds = []string{upperDir, workDir}
	mountPoint = &MountPoint{
		source: source,
		target: target,
		fstype: overlayFsType,
		flags:  overlayFlags,
		data:   overlayData,
	}

	return
}

// GetSource gets the source device of the mount.
func (m *MountPoint) GetSource() string {
	return m.source
}

// GetFSType gets the file-system type of the mount.
func (m *MountPoint) GetFSType() string {
	return m.fstype
}

// GetTarget gets the target directory path of the mount.
func (m *MountPoint) GetTarget() string {
	return m.target
}

// GetFlags gets the flags of the mount.
func (m *MountPoint) GetFlags() uintptr {
	return m.flags
}

// NewChroot creates a new Chroot struct
func NewChroot(rootDir string, isExistingDir bool) *Chroot {
	// get chroot folder
	chrootDir, err := buildpipeline.GetChrootDir(rootDir)
	if err != nil {
		logger.Log.Panicf("Failed to get chroot dir - %s", err.Error())
		return nil
	}

	// create new safechroot
	c := new(Chroot)
	c.rootDir = chrootDir
	if buildpipeline.IsRegularBuild() {
		c.isExistingDir = isExistingDir
	} else {
		// Docker based pipeline recycle chroot =>
		// - chroot always exists
		// - chroot must be cleaned-up before being used
		c.isExistingDir = true
		buildpipeline.CleanupDockerChroot(c.rootDir)
	}
	return c
}

// Initialize initializes a Chroot, creating directories and mount points.
//   - tarPath is an optional path to a tar file that will be extracted at the root of the chroot.
//   - extraDirectories is an optional slice of additional directories that should be created before attempting to
//     mount inside the chroot.
//   - extraMountPoints is an optional slice of additional mount points that should be created inside the chroot,
//     they will automatically be unmounted on a Chroot Close.
//
// This call will block until the chroot initializes successfully.
// Only one Chroot will initialize at a given time.
func (c *Chroot) Initialize(tarPath string, extraDirectories []string, extraMountPoints []*MountPoint,
	includeDefaultMounts bool,
) (err error) {
	// On failed initialization, cleanup all chroot files
	const leaveChrootOnDisk = false

	// Acquire a lock on the global activeChrootsMutex to ensure SIGTERM
	// teardown doesn't happen mid-initialization.
	activeChrootsMutex.Lock()
	defer activeChrootsMutex.Unlock()

	if c.isExistingDir {
		_, err = os.Stat(c.rootDir)
		if os.IsNotExist(err) {
			err = fmt.Errorf("chroot directory (%s) does not exist", c.rootDir)
			return
		}
	} else {
		// Prevent a Chroot from being made on top of an existing directory.
		// Chroot cleanup involves deleting the rootdir, so assume Chroot
		// has exclusive ownership of it.
		_, err = os.Stat(c.rootDir)
		if !os.IsNotExist(err) {
			err = fmt.Errorf("chroot directory (%s) already exists", c.rootDir)
			return
		}

		// Create new root directory
		err = os.MkdirAll(c.rootDir, os.ModePerm)
		if err != nil {
			err = fmt.Errorf("failed to create chroot directory (%s):\n%w", c.rootDir, err)
			return
		}
	}

	// Defer cleanup after it has been confirmed rootDir will not
	// overwrite an existing directory when isExistingDir is set to false.
	defer func() {
		if err != nil {
			if buildpipeline.IsRegularBuild() {
				// mount/unmount is only supported in regular pipeline
				// Best effort cleanup in case mountpoint creation failed mid-way through. We will not try again so treat as final attempt.
				cleanupErr := c.unmountAndRemove(leaveChrootOnDisk, unmountTypeLazy)
				if cleanupErr != nil {
					logger.Log.Warnf("Failed to cleanup chroot (%s) during failed initialization:\n%s", c.rootDir, cleanupErr)
				}
			} else {
				// release chroot dir
				cleanupErr := buildpipeline.ReleaseChrootDir(c.rootDir)
				if cleanupErr != nil {
					logger.Log.Warnf("Failed to release chroot (%s) during failed initialization:\n%s", c.rootDir, cleanupErr)
				}
			}
		}
	}()

	// Extract a given tarball if necessary
	if tarPath != "" {
		err = extractWorkerTar(c.rootDir, tarPath)
		if err != nil {
			err = fmt.Errorf("failed to extract worker tar:\n%w", err)
			return
		}
	}

	// Create extra directories
	for _, dir := range extraDirectories {
		err = os.MkdirAll(filepath.Join(c.rootDir, dir), os.ModePerm)
		if err != nil {
			err = fmt.Errorf("failed to create extra directory inside chroot (%s):\n%w", dir, err)
			return
		}
	}

	// mount is only supported in regular pipeline
	if buildpipeline.IsRegularBuild() {
		// Create kernel mountpoints
		allMountPoints := []*MountPoint{}

		for _, mountPoint := range extraMountPoints {
			if mountPoint.mountBeforeDefaults {
				allMountPoints = append(allMountPoints, mountPoint)
			}
		}

		if includeDefaultMounts {
			allMountPoints = append(allMountPoints, defaultMountPoints()...)
		}

		for _, mountPoint := range extraMountPoints {
			if !mountPoint.mountBeforeDefaults {
				allMountPoints = append(allMountPoints, mountPoint)
			}
		}

		// Assign to `c.mountPoints` now since `Initialize` will call `unmountAndRemove` if an error occurs.
		c.mountPoints = allMountPoints
		c.includeDefaultMounts = includeDefaultMounts

		// Mount with the original unsorted order. Assumes the order of mounts is important.
		err = c.createMountPoints()
		if err != nil {
			err = fmt.Errorf("failed to create mountpoints for chroot:\n%w", err)
			return
		}

		// Mark this chroot as initialized, allowing it to be cleaned up on SIGTERM
		// if requested.
		activeChroots = append(activeChroots, c)
	}

	return
}

// AddDirs copies each directory 'Src' to the relative path chrootRootDir/'Dest' in the chroot.
func (c *Chroot) AddDirs(dirToCopy DirToCopy) (err error) {
	return file.CopyDir(dirToCopy.Src, filepath.Join(c.rootDir, dirToCopy.Dest), dirToCopy.NewDirPermissions,
		dirToCopy.ChildFilePermissions, dirToCopy.MergedDirPermissions)
}

// AddFiles copies each file 'Src' to the relative path chrootRootDir/'Dest' in the chroot.
func (c *Chroot) AddFiles(filesToCopy ...FileToCopy) (err error) {
	return AddFilesToDestination(c.rootDir, filesToCopy...)
}

func AddFilesToDestination(destDir string, filesToCopy ...FileToCopy) error {
	for _, f := range filesToCopy {
		switch {
		case f.Src != "" && f.Content != nil:
			return fmt.Errorf("cannot specify both 'Src' and 'Content' for 'FileToCopy'")

		case f.Src != "":
			err := copyFile(destDir, f)
			if err != nil {
				return err
			}

		case f.Content != nil:
			err := writeFile(destDir, f)
			if err != nil {
				return err
			}

		default:
			return fmt.Errorf("must specify either 'Src' and 'Content' for 'FileToCopy'")
		}
	}

	return nil
}

func copyFile(destDir string, f FileToCopy) error {
	dest := filepath.Join(destDir, f.Dest)
	fileCopyOp := file.NewFileCopyBuilder(f.Src, dest)
	if f.NoDereference {
		fileCopyOp = fileCopyOp.SetNoDereference()
	}
	if f.Permissions != nil {
		fileCopyOp = fileCopyOp.SetFileMode(*f.Permissions)
	}

	err := fileCopyOp.Run()
	if err != nil {
		return fmt.Errorf("failed to copy (%s) to (%s):\n%w", f.Src, f.Dest, err)
	}

	return nil
}

func writeFile(destDir string, f FileToCopy) error {
	dest := filepath.Join(destDir, f.Dest)

	err := file.CreateDestinationDir(dest, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create destination directory (%s):\n%w", dest, err)
	}

	err = file.Write(*f.Content, dest)
	if err != nil {
		return fmt.Errorf("failed to write file (%s):\n%w", f.Dest, err)
	}

	if f.Permissions != nil {
		err = os.Chmod(dest, *f.Permissions)
		if err != nil {
			return fmt.Errorf("failed to set file permissions (%s):\n%w", f.Dest, err)
		}
	}

	return nil
}

func (c *Chroot) WriteFiles() error {
	return nil
}

// CopyOutFile copies file 'srcPath' in the chroot to the host at 'destPath'
func (c *Chroot) CopyOutFile(srcPath string, destPath string) (err error) {
	srcPathFull := filepath.Join(c.rootDir, srcPath)
	err = file.Copy(srcPathFull, destPath)
	if err != nil {
		return fmt.Errorf("failed to copy (%s):\n%w", srcPathFull, err)
	}
	return
}

// MoveOutFile moves file 'srcPath' in the chroot to the host at 'destPath', deleting the 'srcPath' file.
func (c *Chroot) MoveOutFile(srcPath string, destPath string) (err error) {
	srcPathFull := filepath.Join(c.rootDir, srcPath)
	err = file.Move(srcPathFull, destPath)
	if err != nil {
		return fmt.Errorf("failed to move file from (%s) to (%s):\n%w", srcPath, destPath, err)
	}
	return
}

// Run runs a given function inside the Chroot. This function will synchronize
// with all other Chroots to ensure only one Chroot command is executed at a given time.
func (c *Chroot) Run(toRun func() error) (err error) {
	// Only a single chroot can be active at a given time for a single GO application.
	// acquire a global mutex to ensure this behavior.
	inChrootMutex.Lock()
	defer inChrootMutex.Unlock()

	// Alter the environment variables while inside the chroot, upon exit restore them.
	originalEnv := shell.CurrentEnvironment()
	shell.SetEnvironment(defaultChrootEnv)
	defer shell.SetEnvironment(originalEnv)

	err = c.UnsafeRun(toRun)

	return
}

// UnsafeRun runs a given function inside the Chroot. This function will not synchronize
// with other Chroots. The invoker is responsible for ensuring safety.
func (c *Chroot) UnsafeRun(toRun func() error) (err error) {
	const fsRoot = "/"

	originalRoot, err := os.Open(fsRoot)
	if err != nil {
		return
	}
	defer originalRoot.Close()

	cwd, err := os.Getwd()
	if err != nil {
		return
	}
	originalWd, err := os.Open(cwd)
	if err != nil {
		return
	}
	defer originalWd.Close()

	logger.Log.Debugf("Entering Chroot: '%s'", c.rootDir)
	err = unix.Chroot(c.rootDir)
	if err != nil {
		return
	}
	defer c.restoreRoot(originalRoot, originalWd)

	err = os.Chdir(fsRoot)
	if err != nil {
		return
	}

	err = toRun()
	return
}

// RootDir returns the Chroot's root directory.
func (c *Chroot) RootDir() string {
	return c.rootDir
}

// Close will unmount the chroot and cleanup its files.
// This call will block until the chroot cleanup runs.
// Only one Chroot will close at a given time.
func (c *Chroot) Close(leaveOnDisk bool) (err error) {
	// Acquire a lock on the global activeChrootsMutex to ensure SIGTERM
	// teardown doesn't happen mid-close.
	activeChrootsMutex.Lock()
	defer activeChrootsMutex.Unlock()

	if buildpipeline.IsRegularBuild() {
		index := -1
		for i, chroot := range activeChroots {
			if chroot == c {
				index = i
				break
			}
		}

		if index < 0 {
			// Already closed.
			return
		}

		// Stops processes that are running inside the chroot (e.g. gpg-agent).
		// This is to avoid leaving folders like /dev mounted when the chroot folder is forcefully deleted in cleanup.
		err = c.stopRunningProcesses()
		if err != nil {
			// Don't want to leave a stale root if GPG components fail to exit. Logging a Warn and letting close continue...
			logger.Log.Warnf("Failed to stop running processes while tearing down the (%s) chroot:\n%s", c.rootDir, err)
		}

		// mount is only supported in regular pipeline
		err = c.unmountAndRemove(leaveOnDisk, unmountTypeNormal)
		if err != nil {
			logger.Log.Warnf("Chroot cleanup failed, will retry with lazy unmount. Error: %s", err)
			err = c.unmountAndRemove(leaveOnDisk, unmountTypeLazy)
		}
		if err == nil {
			const emptyLen = 0
			// Remove this chroot from the list of active ones since it has now been cleaned up.
			// Create a new slice that is -1 capacity of the current activeChroots.
			newActiveChroots := make([]*Chroot, emptyLen, len(activeChroots)-1)
			newActiveChroots = append(newActiveChroots, activeChroots[:index]...)
			newActiveChroots = append(newActiveChroots, activeChroots[index+1:]...)
			activeChroots = newActiveChroots
		}
	} else {
		// release chroot dir
		err = buildpipeline.ReleaseChrootDir(c.rootDir)
	}

	return
}

// registerSIGTERMCleanup will register SIGTERM handling to force all Chroots
// to Close before exiting the application.
func registerSIGTERMCleanup() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, unix.SIGINT, unix.SIGTERM)
	go cleanupAllChrootsOnSignal(signals)
}

// cleanupAllChrootsOnSignal will cleanup all chroots on an os signal.
func cleanupAllChrootsOnSignal(signals chan os.Signal) {
	sig := <-signals
	logger.Log.Error(sig)

	cleanupAllChroots()

	os.Exit(1)
}

// cleanupAllChroots will unmount and cleanup all running chroots.
// *NOTE*: invocation of this method assumes application teardown. It will leave
// Chroot in state where all operations (Initialize/Run/Close) will block indefinitely.
func cleanupAllChroots() {
	// This code blocks all Chroot operations,
	// and frees the underlying OS handles associated with the chroots (unmounting them).
	//
	// However, it does not actually free the Chroot objects created by other goroutines, as they hold reference to them.
	// Thus it could leave other go routines' Chroots in a bad state, where the routine believes the chroot is in-fact initialized,
	// but really it has already been cleaned up.

	const (
		// On cleanup, remove all chroot files
		leaveChrootOnDisk = false
		// On cleanup SIGKILL all children processes.
		stopSignal = unix.SIGKILL
	)

	// Acquire and permanently hold the global activeChrootsMutex to ensure no
	// new Chroots are initialized or unmounted during this teardown routine
	logger.Log.Info("Waiting for outstanding chroot initialization and cleanup to finish")
	activeChrootsMutex.Lock()

	// Acquire and permanently hold the global inChrootMutex lock to ensure this application is not
	// inside any Chroot.
	logger.Log.Info("Waiting for outstanding chroot commands to finish")
	shell.PermanentlyStopAllChildProcesses(stopSignal)
	inChrootMutex.Lock()

	// mount is only supported in regular pipeline
	failedToUnmount := false
	if buildpipeline.IsRegularBuild() {
		// Cleanup chroots in LIFO order incase any are interdependent (e.g. nested safe chroots)
		logger.Log.Info("Cleaning up all active chroots")
		for i := len(activeChroots) - 1; i >= 0; i-- {
			logger.Log.Infof("Cleaning up chroot (%s)", activeChroots[i].rootDir)
			err := activeChroots[i].unmountAndRemove(leaveChrootOnDisk, unmountTypeLazy)
			// Perform best effort cleanup: unmount as many chroots as possible,
			// even if one fails.
			if err != nil {
				logger.Log.Errorf("Failed to unmount chroot (%s)", activeChroots[i].rootDir)
				failedToUnmount = true
			}
		}
	}

	if failedToUnmount {
		logger.Log.Fatalf("Failed to unmount a chroot, manual unmount required. See above errors for details on which mounts failed.")
	} else {
		logger.Log.Info("Cleanup finished")
	}
}

// unmountAndRemove retries to unmount directories that were mounted into
// the chroot until the unmounts succeed or too many failed attempts.
// This is to avoid leaving folders like /dev mounted when the chroot folder is forcefully deleted in cleanup.
// Iff all mounts were successfully unmounted, the chroot's root directory will be removed if requested.
// If doLazyUnmount is true, use the lazy unmount flag which will allow the unmount to succeed even if the mount point is busy.
func (c *Chroot) unmountAndRemove(leaveOnDisk, lazyUnmount bool) (err error) {
	const (
		retryDuration      = time.Second
		totalAttempts      = 3
		unmountFlagsNormal = 0
		// Do a lazy unmount as a fallback. This will allow the unmount to succeed even if the mount point is busy.
		// This is to avoid leaving folders like /dev mounted if the chroot folder is forcefully deleted by the user. Even
		// if the mount is busy at least it will be detached from the filesystem and will not damage the host.
		unmountFlagsLazy = unix.MNT_DETACH
	)
	unmountFlags := unmountFlagsNormal
	if lazyUnmount {
		unmountFlags = unmountFlagsLazy
	}

	// Unmount in the reverse order of mounting to ensure that any nested mounts are unraveled in the correct order.
	for i := len(c.mountPoints) - 1; i >= 0; i-- {
		mountPoint := c.mountPoints[i]

		fullPath := filepath.Join(c.rootDir, mountPoint.target)

		var exists bool
		exists, err = file.PathExists(fullPath)
		if err != nil {
			err = fmt.Errorf("failed to check if mount point (%s) exists. Error: %s", fullPath, err)
			return
		}
		if !exists {
			logger.Log.Debugf("Skipping unmount of (%s) because path doesn't exist", fullPath)
			continue
		}

		var isMounted bool
		isMounted, err = mountinfo.Mounted(fullPath)
		if err != nil {
			err = fmt.Errorf("failed to check if mount point (%s) is mounted. Error: %s", fullPath, err)
			return
		}
		if !isMounted {
			logger.Log.Debugf("Skipping unmount of (%s) because it is not mounted", fullPath)
			continue
		}

		logger.Log.Debugf("Unmounting (%s)", fullPath)

		// Skip mount points if they were not successfully created
		if !mountPoint.isMounted {
			continue
		}

		_, err = retry.RunWithExpBackoff(context.Background(), func() error {
			logger.Log.Debugf("Calling unmount on path(%s) with flags (%v)", fullPath, unmountFlags)
			umountErr := unix.Unmount(fullPath, unmountFlags)
			return umountErr
		}, totalAttempts, retryDuration, 2.0)
		if err != nil {
			err = fmt.Errorf("failed to unmount (%s):\n%w", fullPath, err)
			return
		}
	}

	if !leaveOnDisk {
		err = os.RemoveAll(c.rootDir)
	}

	return
}

// defaultMountPoints returns a new copy of the default mount points used by a functional chroot
func defaultMountPoints() []*MountPoint {
	return []*MountPoint{
		{
			target: "/dev",
			fstype: "devtmpfs",
		},
		{
			target: "/proc",
			fstype: "proc",
		},
		{
			target: "/sys",
			fstype: "sysfs",
		},
		{
			target: "/run",
			fstype: "tmpfs",
		},
		{
			target: "/dev/pts",
			fstype: "devpts",
			data:   "gid=5,mode=620",
		},
	}
}

// restoreRoot will restore the original root of the GO application, cleaning up
// after the run command. Will panic on error.
func (c *Chroot) restoreRoot(originalRoot, originalWd *os.File) {
	logger.Log.Debugf("Exiting Chroot: '%s'", c.rootDir)

	err := originalRoot.Chdir()
	if err != nil {
		logger.Log.Panicf("Failed to change directory to original root. Error: %s", err)
	}

	err = unix.Chroot(".")
	if err != nil {
		logger.Log.Panicf("Failed to restore original chroot. Error: %s", err)
	}

	err = originalWd.Chdir()
	if err != nil {
		logger.Log.Panicf("Failed to change directory to original root. Error: %s", err)
	}

	return
}

// createMountPoints will create a provided list of mount points
func (c *Chroot) createMountPoints() (err error) {
	for _, mountPoint := range c.mountPoints {
		fullPath := filepath.Join(c.rootDir, mountPoint.target)
		logger.Log.Debugf("Mounting: source: (%s), target: (%s), fstype: (%s), flags: (%#x), data: (%s)",
			mountPoint.source, fullPath, mountPoint.fstype, mountPoint.flags, mountPoint.data)

		err = os.MkdirAll(fullPath, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create directory (%s)", fullPath)
		}

		err = unix.Mount(mountPoint.source, fullPath, mountPoint.fstype, mountPoint.flags, mountPoint.data)
		if err != nil {
			return fmt.Errorf("failed to mount (%s) to (%s):\n%w", mountPoint.source, fullPath, err)
		}

		mountPoint.isMounted = true
	}

	return
}

// extractWorkerTar uses tar with gzip or pigz to setup a chroot directory using a rootfs tar
func extractWorkerTar(chroot string, workerTar string) (err error) {
	gzipTool, err := systemdependency.GzipTool()
	if err != nil {
		return err
	}

	logger.Log.Debugf("Using (%s) to extract tar", gzipTool)
	_, _, err = shell.Execute("tar", "-I", gzipTool, "-xf", workerTar, "-C", chroot)
	return
}

// GetMountPoints gets a copy of the list of mounts the Chroot was initialized with.
func (c *Chroot) GetMountPoints() []*MountPoint {
	// Create a copy of the list so that the caller can't mess with the list.
	mountPoints := append([]*MountPoint(nil), c.mountPoints...)
	return mountPoints
}

// Stops all the processes running within the chroot.
func (c *Chroot) stopRunningProcesses() error {
	// Get a list of all the processes that have a file open in the root directory.
	records, err := processes.GetProcessesUsingPath(c.rootDir)
	if err != nil {
		return err
	}

	errs := []error(nil)
	for _, record := range records {
		if record.ProcessRoot == c.RootDir() {
			// This process was started under the chroot.
			// So, stop it.
			logger.Log.Debugf("Found chroot process: Pid=%d, Name=%s.", record.ProcessId, record.ProcessName)

			err := processes.StopProcessById(record.ProcessId)
			if err != nil {
				errs = append(errs, err)
			}
		} else {
			// There is a host process poking around the root directory. This is very unlikely to be started by us. And
			// we don't know what it might be doing or why.
			// So, just warn the user and hopes it stops soon.
			logger.Log.Warnf("Found host process using chroot files: Pid=%d, Name=%s.", record.ProcessId,
				record.ProcessName)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}
