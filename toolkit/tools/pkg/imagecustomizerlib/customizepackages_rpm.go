package imagecustomizerlib

// AzureLinuxDistroConfig implements DistroConfig for Azure Linux
type AzureLinuxDistroConfig struct{}

func (d *AzureLinuxDistroConfig) GetPackageManagerBinary() string { return "tdnf" }
func (d *AzureLinuxDistroConfig) GetPackageType() string          { return "rpm" }
func (d *AzureLinuxDistroConfig) GetReleaseVersion() string       { return "3.0" }
func (d *AzureLinuxDistroConfig) UsesCacheOnly() bool             { return true }
func (d *AzureLinuxDistroConfig) UsesInstallRoot() bool           { return true }
func (d *AzureLinuxDistroConfig) GetConfigFile() string           { return customTdnfConfRelPath }
func (d *AzureLinuxDistroConfig) GetDistroName() string           { return "azurelinux" }
func (d *AzureLinuxDistroConfig) GetPackageSourceDir() string     { return rpmsMountParentDirInChroot }

// FedoraDistroConfig implements DistroConfig for Fedora
type FedoraDistroConfig struct{}

func (d *FedoraDistroConfig) GetPackageManagerBinary() string { return "dnf" }
func (d *FedoraDistroConfig) GetPackageType() string          { return "rpm" }
func (d *FedoraDistroConfig) GetReleaseVersion() string       { return "42" }
func (d *FedoraDistroConfig) UsesCacheOnly() bool             { return false }
func (d *FedoraDistroConfig) UsesInstallRoot() bool           { return true }
func (d *FedoraDistroConfig) GetConfigFile() string           { return "etc/dnf/dnf.conf" }
func (d *FedoraDistroConfig) GetDistroName() string           { return "fedora" }
func (d *FedoraDistroConfig) GetPackageSourceDir() string     { return rpmsMountParentDirInChroot }

// Helper functions for building package manager command arguments
func buildInstallArgs(distroConfig DistroConfig, packages []string, repoDir string, useToolsChroot bool) []string {
	args := []string{"install", "--assumeyes"}

	// Add distro-specific flags
	if distroConfig.UsesCacheOnly() {
		args = append(args, "--cacheonly")
	}

	if repoDir != "" {
		args = append(args, "--setopt=reposdir="+repoDir)
	}

	args = append(args, packages...)

	if useToolsChroot && distroConfig.UsesInstallRoot() {
		args = append([]string{"--releasever=" + distroConfig.GetReleaseVersion(), "--installroot=/" + toolsRootImageDir}, args...)
	}

	return args
}

func buildUpdateArgs(distroConfig DistroConfig, packages []string, repoDir string, useToolsChroot bool) []string {
	args := []string{"update", "--assumeyes"}

	// Add distro-specific flags
	if distroConfig.UsesCacheOnly() {
		args = append(args, "--cacheonly")
	}

	if repoDir != "" {
		args = append(args, "--setopt=reposdir="+repoDir)
	}

	args = append(args, packages...)

	if useToolsChroot && distroConfig.UsesInstallRoot() {
		args = append([]string{"--releasever=" + distroConfig.GetReleaseVersion(), "--installroot=/" + toolsRootImageDir}, args...)
	}

	return args
}

func buildRemoveArgs(distroConfig DistroConfig, packages []string, useToolsChroot bool) []string {
	args := []string{"remove", "--assumeyes", "--disablerepo", "*"}
	args = append(args, packages...)

	if useToolsChroot && distroConfig.UsesInstallRoot() {
		args = append([]string{"--releasever=" + distroConfig.GetReleaseVersion(), "--installroot=/" + toolsRootImageDir}, args...)
	}

	return args
}

func buildUpdateAllArgs(distroConfig DistroConfig, repoDir string, useToolsChroot bool) []string {
	args := []string{"update", "--assumeyes"}

	// Add distro-specific flags
	if distroConfig.UsesCacheOnly() {
		args = append(args, "--cacheonly")
	}

	if repoDir != "" {
		args = append(args, "--setopt=reposdir="+repoDir)
	}

	if useToolsChroot && distroConfig.UsesInstallRoot() {
		args = append([]string{"--releasever=" + distroConfig.GetReleaseVersion(), "--installroot=/" + toolsRootImageDir}, args...)
	}

	return args
}

func buildCleanCacheArgs(distroConfig DistroConfig, useToolsChroot bool) []string {
	args := []string{"clean", "all"}

	if useToolsChroot && distroConfig.UsesInstallRoot() {
		args = append([]string{"--releasever=" + distroConfig.GetReleaseVersion(), "--installroot=/" + toolsRootImageDir}, args...)
	}

	return args
}

func buildRefreshMetadataArgs(distroConfig DistroConfig, repoDir string, useToolsChroot bool) []string {
	args := []string{"makecache"}

	if repoDir != "" {
		args = append(args, "--setopt=reposdir="+repoDir)
	}

	// For Fedora, ensure cache is kept after makecache
	if distroConfig.GetDistroName() == "fedora" {
		args = append(args, "--setopt=keepcache=1")
	}

	if useToolsChroot && distroConfig.UsesInstallRoot() {
		args = append([]string{"--releasever=" + distroConfig.GetReleaseVersion(), "--installroot=/" + toolsRootImageDir}, args...)
	}

	return args
}
