// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"golang.org/x/sys/unix"
)

var (
	ErrEtcOverlayAssemble = NewImageCustomizerError("EtcOverlay:Assemble", "failed to assemble /etc overlay")
	ErrEtcOverlayFinalize = NewImageCustomizerError("EtcOverlay:Finalize", "failed to finalize /etc overlay")
)

const (
	// The /etc baseline directory, relative to the image root. At runtime, ACL composes /etc as an
	// overlay whose read-only lower layer is this directory and whose writable upper layer is the
	// physical /etc on the root partition.
	aclEtcBaselineDir = "usr/share/distro/etc"

	// Scratch directory holding the overlay's work directory for the duration of customization.
	etcOverlayWorkDirName = ".ic-etc-work"

	// Temporary directory holding the merged /etc snapshot while folding.
	etcOverlayFoldTmpName = ".ic-etc-fold"
)

// etcOverlayFoldExclusions lists paths that are relative to /etc that must never be folded into the
// baseline. machine-id is per-machine identity: the ACL image build removes it before populating
// the baseline so that systemd sees a missing file on first boot and runs its first-boot logic.
var etcOverlayFoldExclusions = []string{
	"machine-id",
}

// EtcOverlay is a distro's /etc overlay, assembled over an image's /etc for the duration of OS
// customization. See DistroHandler.SetupEtcOverlay.
type EtcOverlay struct {
	// Host path of the image's /etc: the overlay's mountpoint and its writable upper layer.
	etcDir string
	// Host path of the /etc baseline, the overlay's read-only lower layer.
	baselineDir string
	// Host path of the overlay's scratch work directory.
	workDir string
	mount   *safemount.Mount
	// Whether the baseline's filesystem is writable, so the merged /etc can be folded into
	// the baseline when the overlay is finalized.
	foldable bool
	done     bool
}

// newAclEtcOverlay assembles ACL's /etc overlay over the image's /etc, mirroring the mount the ACL
// initrd creates at runtime: the baseline is the lower layer and the physical /etc is the upper.
func newAclEtcOverlay(ctx context.Context, rootDir string) (*EtcOverlay, error) {
	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "assemble_etc_overlay")
	defer span.End()

	overlay, err := newAclEtcOverlayHelper(rootDir)
	if err != nil {
		return nil, fmt.Errorf("%w:\n%w", ErrEtcOverlayAssemble, err)
	}

	return overlay, nil
}

func newAclEtcOverlayHelper(rootDir string) (*EtcOverlay, error) {
	baselineDir := filepath.Join(rootDir, aclEtcBaselineDir)
	etcDir := filepath.Join(rootDir, "etc")
	workDir := filepath.Join(rootDir, etcOverlayWorkDirName)

	baselineExists, err := file.DirExists(baselineDir)
	if err != nil {
		return nil, fmt.Errorf("failed to check for /etc baseline directory (%s):\n%w", aclEtcBaselineDir, err)
	}
	if !baselineExists {
		return nil, fmt.Errorf("image has no /etc baseline directory (%s)", aclEtcBaselineDir)
	}

	// The baseline lives on the /usr filesystem, which is only mounted read-write when the image is
	// customized with reinitializeVerity: all. When it is writable, finalizing the overlay folds the
	// merged /etc into the baseline, while when it is read-only, the /etc changes are left on the
	// root partition, just like changes made on a booted system.
	foldable := false
	err = unix.Access(baselineDir, unix.W_OK)
	switch {
	case err == nil:
		foldable = true
	case errors.Is(err, unix.EROFS):
		foldable = false
	default:
		return nil, fmt.Errorf("failed to check /etc baseline writability:\n%w", err)
	}

	if foldable {
		logger.Log.Infof("Assembling /etc overlay")
	} else {
		logger.Log.Infof("Assembling /etc overlay (read-only baseline; /etc changes will stay on the root partition)")
	}

	err = os.MkdirAll(etcDir, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to create /etc directory:\n%w", err)
	}

	// Remove scratch state left behind by an interrupted previous run.
	err = os.RemoveAll(workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to remove stale overlay work directory (%s):\n%w", workDir, err)
	}

	err = os.MkdirAll(workDir, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to create overlay work directory (%s):\n%w", workDir, err)
	}

	// The physical /etc is both the mountpoint and the upper layer, exactly as in the runtime
	// mount (the kernel resolves the layer paths before the new mount covers /etc). redirect_dir
	// enables directory renames, metacopy is disabled so upper entries always carry full data.
	// Both match the options ACL's initrd uses.
	options := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s,redirect_dir=on,metacopy=off",
		baselineDir, etcDir, workDir)

	mount, err := safemount.NewMount("overlay", etcDir, "overlay", unix.MS_NOATIME, options,
		false /*makeAndDeleteDir*/)
	if err != nil {
		removeErr := os.RemoveAll(workDir)
		if removeErr != nil {
			logger.Log.Warnf("Failed to remove overlay work directory (%s): %v", workDir, removeErr)
		}
		return nil, err
	}

	overlay := &EtcOverlay{
		etcDir:      etcDir,
		baselineDir: baselineDir,
		workDir:     workDir,
		mount:       mount,
		foldable:    foldable,
	}
	return overlay, nil
}

func (o *EtcOverlay) foldTmpDir() string {
	return filepath.Join(filepath.Dir(o.baselineDir), etcOverlayFoldTmpName)
}

// Finalize unmounts the overlay. When the baseline is writable, the merged /etc is first folded
// into the baseline and the physical /etc is left empty; otherwise the accumulated /etc changes
// remain on the root partition, matching what the same changes would produce on a booted system.
func (o *EtcOverlay) Finalize(ctx context.Context) error {
	if o == nil {
		return nil
	}

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "finalize_etc_overlay")
	defer span.End()

	var err error
	if o.foldable {
		err = o.foldAndUnmountHelper()
	} else {
		err = o.unmountHelper()
	}
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrEtcOverlayFinalize, err)
	}

	o.done = true
	return nil
}

func (o *EtcOverlay) unmountHelper() error {
	logger.Log.Infof("Unmounting /etc overlay (leaving /etc changes on the root partition)")

	err := o.mount.CleanClose()
	if err != nil {
		return err
	}

	err = os.RemoveAll(o.workDir)
	if err != nil {
		return fmt.Errorf("failed to remove overlay work directory (%s):\n%w", o.workDir, err)
	}

	return nil
}

func (o *EtcOverlay) foldAndUnmountHelper() error {
	logger.Log.Infof("Folding /etc overlay into the image's /etc baseline")

	// Snapshot the merged /etc through the mountpoint, so whiteouts and directory redirects are
	// already resolved. Copying the directory itself, rather than its contents, preserves
	// ownership, permissions, timestamps, hardlinks, symlinks, and xattrs for /etc itself too.
	foldTmpDir := o.foldTmpDir()
	err := os.RemoveAll(foldTmpDir)
	if err != nil {
		return fmt.Errorf("failed to remove stale fold directory (%s):\n%w", foldTmpDir, err)
	}

	// `-a` ensures unix permissions, extended attributes (including SELinux), and sub-directories
	// (-r) are copied. `--no-dereference` ensures that symlinks are copied as symlinks.
	err = shell.NewExecBuilder("cp", "--verbose", "-a", "--no-dereference", o.etcDir, foldTmpDir).
		LogLevel(logrus.TraceLevel, logrus.DebugLevel).
		ErrorStderrLines(1).
		Execute()
	if err != nil {
		return fmt.Errorf("failed to snapshot merged /etc:\n%w", err)
	}

	for _, relPath := range etcOverlayFoldExclusions {
		err = os.RemoveAll(filepath.Join(foldTmpDir, relPath))
		if err != nil {
			return fmt.Errorf("failed to exclude (%s) from the /etc baseline:\n%w", relPath, err)
		}
	}

	err = o.mount.CleanClose()
	if err != nil {
		return err
	}

	// Swap the snapshot in as the new baseline. Both paths are on the same filesystem, so the
	// window without a baseline is two renames wide.
	oldBaselineDir := o.baselineDir + ".old"
	err = os.RemoveAll(oldBaselineDir)
	if err != nil {
		return fmt.Errorf("failed to remove stale directory (%s):\n%w", oldBaselineDir, err)
	}

	err = os.Rename(o.baselineDir, oldBaselineDir)
	if err != nil {
		return fmt.Errorf("failed to move aside old /etc baseline:\n%w", err)
	}

	err = os.Rename(foldTmpDir, o.baselineDir)
	if err != nil {
		return fmt.Errorf("failed to install new /etc baseline:\n%w", err)
	}

	err = os.RemoveAll(oldBaselineDir)
	if err != nil {
		return fmt.Errorf("failed to remove old /etc baseline:\n%w", err)
	}

	// Empty the physical /etc: its previous contents are part of the new baseline now.
	err = file.RemoveDirectoryContents(o.etcDir)
	if err != nil {
		return fmt.Errorf("failed to empty /etc directory:\n%w", err)
	}

	err = os.RemoveAll(o.workDir)
	if err != nil {
		return fmt.Errorf("failed to remove overlay work directory (%s):\n%w", o.workDir, err)
	}

	return nil
}

// Close unmounts the overlay without folding. Intended as deferred error-path cleanup, the image
// is expected to be discarded. No-op after a successful Finalize and on a nil receiver.
func (o *EtcOverlay) Close() {
	if o == nil || o.done {
		return
	}

	o.mount.Close()

	for _, dir := range []string{o.workDir, o.foldTmpDir()} {
		err := os.RemoveAll(dir)
		if err != nil {
			logger.Log.Warnf("Failed to remove overlay directory (%s): %v", dir, err)
		}
	}

	o.done = true
}
