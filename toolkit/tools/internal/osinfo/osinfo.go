package osinfo

import (
	"os"
	"strings"
)

// Function to get the distribution and version of the host machine
func GetDistroAndVersion() (string, string) {
	output, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "Unknown Distro", "Unknown Version"
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
