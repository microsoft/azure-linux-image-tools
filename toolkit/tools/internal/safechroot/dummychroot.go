// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package safechroot

// DummyChroot is a placeholder that implements ChrootInterface.
type DummyChroot struct {
}

func (d *DummyChroot) RootDir() string {
	return "/"
}

func (d *DummyChroot) ChrootDir() string {
	// No chroot neccessary when executing subprocesses.
	return ""
}

func (d *DummyChroot) AddFiles(filesToCopy ...FileToCopy) (err error) {
	return AddFilesToDestination(d.RootDir(), filesToCopy...)
}
