// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/google/uuid"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"golang.org/x/sys/unix"
)

type stagedErofs struct {
	partition diskutils.PartitionInfo
	directory string
	image     string
}

type erofsFilesystemEdits struct {
	stages []stagedErofs
}

func (s *erofsFilesystemEdits) prepare(connection *imageconnection.ImageConnection,
	mounts []*safechroot.MountPoint, partitions []diskutils.PartitionInfo, buildDir string,
	verityMetadata []verityDeviceMetadata,
) error {
	for i, mount := range mounts {
		if mount.GetFlags()&unix.MS_RDONLY != 0 {
			continue
		}
		index := slices.IndexFunc(partitions, func(p diskutils.PartitionInfo) bool {
			return p.Type == "part" && p.Path == mount.GetSource()
		})
		if index < 0 {
			continue
		}
		partition := partitions[index]
		if partition.FileSystemType != "erofs" {
			continue
		}
		if mount.GetTarget() == "/" {
			return fmt.Errorf("customizing an EROFS root filesystem is not supported")
		}
		for _, metadata := range verityMetadata {
			if metadata.dataPartUuid == partition.PartUuid &&
				(metadata.formatSettings.IsInlineVerity() ||
					(metadata.formatSettings.dataSizeBytes != 0 &&
						metadata.formatSettings.dataSizeBytes != partition.SizeInBytes)) {
				return fmt.Errorf("EROFS customization requires separate, whole-partition verity (%s)", partition.Path)
			}
		}
		stageIndex := slices.IndexFunc(s.stages, func(stage stagedErofs) bool {
			return stage.partition.Path == partition.Path
		})
		if stageIndex < 0 {
			stage, err := stageErofs(connection, partition, buildDir)
			if err != nil {
				return err
			}
			s.stages = append(s.stages, stage)
			stageIndex = len(s.stages) - 1
		}
		mounts[i] = safechroot.NewMountPoint(s.stages[stageIndex].directory,
			mount.GetTarget(), "none", mount.GetFlags()|unix.MS_BIND, "")
	}
	return nil
}

func (s *erofsFilesystemEdits) finalize() error {
	for _, stage := range s.stages {
		args := []string{"-b", "4096", "-U", stage.partition.Uuid}
		if stage.partition.Label != "" {
			args = append(args, "-L", stage.partition.Label)
		}
		args = append(args, "--", stage.image, stage.directory)
		if err := shell.ExecuteLive(true, "mkfs.erofs", args...); err != nil {
			return fmt.Errorf("failed to repack EROFS (%s):\n%w", stage.partition.Path, err)
		}
		if err := checkFileSystem("erofs", stage.image); err != nil {
			return err
		}
		if err := verifyPackedErofs(stage); err != nil {
			return err
		}
		if err := writeErofsPartition(stage.image, stage.partition); err != nil {
			return err
		}
		if err := checkFileSystem("erofs", stage.partition.Path); err != nil {
			return err
		}
	}
	return nil
}

func stageErofs(connection *imageconnection.ImageConnection, partition diskutils.PartitionInfo,
	buildDir string,
) (stagedErofs, error) {
	for _, tool := range []string{"mkfs.erofs", "fsck.erofs"} {
		if _, err := exec.LookPath(tool); err != nil {
			return stagedErofs{}, fmt.Errorf("EROFS customization requires erofs-utils (%s):\n%w", tool, err)
		}
	}
	if _, err := uuid.Parse(partition.Uuid); err != nil {
		return stagedErofs{}, fmt.Errorf("invalid EROFS filesystem UUID (%s):\n%w", partition.Uuid, err)
	}
	if err := checkFileSystem("erofs", partition.Path); err != nil {
		return stagedErofs{}, err
	}
	dir, err := os.MkdirTemp(buildDir, "erofs-customize-")
	if err != nil {
		return stagedErofs{}, err
	}
	connection.AddOwnedDirectories(dir)
	stage := stagedErofs{
		partition: partition,
		directory: filepath.Join(dir, "tree"),
		image:     filepath.Join(dir, "sealed.erofs"),
	}
	if err := os.Mkdir(stage.directory, 0o700); err != nil {
		return stagedErofs{}, err
	}
	source := filepath.Join(dir, "source")
	mount, err := safemount.NewMount(partition.Path, source, "erofs", unix.MS_RDONLY, "", true)
	if err != nil {
		return stagedErofs{}, err
	}
	defer mount.Close()
	// Copy the directory itself, including root metadata, without dereferencing symlinks.
	if err := shell.ExecuteLive(true, "cp", "-a", "--preserve=all", "--reflink=auto",
		"--", source+"/.", stage.directory); err != nil {
		return stagedErofs{}, fmt.Errorf("failed to stage EROFS (%s):\n%w", partition.Path, err)
	}
	if err := verifyErofsTree(source, stage.directory); err != nil {
		return stagedErofs{}, fmt.Errorf("EROFS staging verification failed:\n%w", err)
	}
	if err := mount.CleanClose(); err != nil {
		return stagedErofs{}, err
	}
	return stage, nil
}

func verifyPackedErofs(stage stagedErofs) error {
	loopback, err := safeloopback.NewLoopback(stage.image)
	if err != nil {
		return err
	}
	defer loopback.Close()
	mountPath := filepath.Join(filepath.Dir(stage.image), "verify")
	mount, err := safemount.NewMount(loopback.DevicePath(), mountPath, "erofs", unix.MS_RDONLY, "", true)
	if err != nil {
		return err
	}
	defer mount.Close()
	if err := verifyErofsTree(stage.directory, mountPath); err != nil {
		return fmt.Errorf("EROFS packed-image verification failed:\n%w", err)
	}
	if err := mount.CleanClose(); err != nil {
		return err
	}
	return loopback.CleanClose()
}

func writeErofsPartition(image string, partition diskutils.PartitionInfo) error {
	input, err := os.Open(image)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	if info.Size() <= 0 || uint64(info.Size()) > partition.SizeInBytes {
		return fmt.Errorf("EROFS image size (%d) exceeds partition capacity (%d) or is empty",
			info.Size(), partition.SizeInBytes)
	}
	output, err := os.OpenFile(partition.Path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer output.Close()
	deviceInfo, err := output.Stat()
	if err != nil {
		return err
	}
	if deviceInfo.Mode()&os.ModeDevice == 0 || deviceInfo.Mode()&os.ModeCharDevice != 0 {
		return fmt.Errorf("EROFS destination is not a block device (%s)", partition.Path)
	}
	if _, err := io.CopyN(output, input, info.Size()); err != nil {
		return fmt.Errorf("failed to write EROFS partition (%s):\n%w", partition.Path, err)
	}
	// Do not retain old payload bytes beyond the new filesystem in the verity-covered partition.
	zeros := make([]byte, 1024*1024)
	for remaining := partition.SizeInBytes - uint64(info.Size()); remaining > 0; {
		size := min(remaining, uint64(len(zeros)))
		n, err := output.Write(zeros[:size])
		if err != nil {
			return fmt.Errorf("failed to clear EROFS partition tail (%s):\n%w", partition.Path, err)
		}
		if uint64(n) != size {
			return io.ErrShortWrite
		}
		remaining -= size
	}
	return output.Sync()
}
