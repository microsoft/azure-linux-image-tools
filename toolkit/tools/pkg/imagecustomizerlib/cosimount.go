package imagecustomizerlib

import (
	"fmt"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

type MountPount struct {
	path string
}

func NewLoopDevMount(image string, location string) (*MountPount, error) {
	log.Debugf("Mounting %s at %s", image, location)
	err := exec.Command("mount", "-o", "loop,ro", image, location).Run()
	if err != nil {
		return nil, fmt.Errorf("failed to mount %s at %s: %w", image, location, err)
	}
	return &MountPount{path: location}, nil
}

func (m MountPount) Path() string {
	return m.path
}

func (m MountPount) Close() error {
	return exec.Command("umount", m.path).Run()
}
