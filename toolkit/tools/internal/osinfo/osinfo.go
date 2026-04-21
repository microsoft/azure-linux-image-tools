package osinfo

import (
	"os"
	"path/filepath"
	"strings"
)

// Function to get the distribution and version of the host machine
func GetDistroAndVersion(rootDir string) (string, string) {
	output, err := os.ReadFile(filepath.Join(rootDir, "etc/os-release"))
	if err != nil {
		// Fall back to /usr/lib/os-release per the os-release(5) spec.
		output, err = os.ReadFile(filepath.Join(rootDir, "usr/lib/os-release"))
		if err != nil {
			return "Unknown Distro", "Unknown Version"
		}
	}

	lines := strings.Split(string(output), "\n")
	distro := "Unknown Distro"
	version := "Unknown Version"

	for _, line := range lines {
		if strings.HasPrefix(line, "NAME=") {
			distro = strings.Trim(strings.TrimPrefix(line, "NAME="), "\"")
		} else if strings.HasPrefix(line, "VERSION=") {
			version = strings.Trim(strings.TrimPrefix(line, "VERSION="), "\"")
		}
	}

	return distro, version
}
