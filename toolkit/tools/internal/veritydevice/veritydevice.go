// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package veritydevice

import (
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
)

type VerityDevice struct {
	name   string
	closed bool
}

func OpenVerityDevice(name string, dataDevice string, hashDevice string, hashOffset uint64, rootHash string,
) (*VerityDevice, error) {
	verityDevice := &VerityDevice{
		name: name,
	}

	err := shell.NewExecBuilder("veritysetup", "open", fmt.Sprintf("--hash-offset=%d", hashOffset), dataDevice, name,
		hashDevice, rootHash).
		ErrorStderrLines(1).
		Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to open dm-verity device (name='%s', dataDevice='%s')", name, dataDevice)
	}

	return verityDevice, nil
}

func (d *VerityDevice) CleanClose() error {
	return d.close(false /*async*/)
}

func (d *VerityDevice) Close() {
	err := d.close(true /*async*/)
	if err != nil {
		logger.Log.Warnf("Error: %s", err)
	}
}

func (d *VerityDevice) close(async bool) error {
	if d.closed {
		return nil
	}

	args := []string{"close", d.name}
	if async {
		// Request the device close once there is no more users. This is useful for when downstream mounts were also
		// deferred closed.
		//
		// The '--deferred' arg was added to 'veritysetup close' in v2.4.0.
		// Distros:
		// - Azure Linux 3.0: v2.4.3
		// - Ubuntu 22.04: v2.4.3
		args = append(args, "--deferred")
	}

	err := shell.NewExecBuilder("veritysetup", args...).
		ErrorStderrLines(1).
		Execute()
	if err != nil {
		return fmt.Errorf("failed to close dm-verity device (name='%s')", d.name)
	}
	d.closed = true

	return nil
}

func (d *VerityDevice) Name() string {
	return d.name
}

func (d *VerityDevice) Path() string {
	return "/dev/mapper/" + d.name
}
