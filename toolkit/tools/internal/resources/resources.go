// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package resources

import (
	"embed"
)

const (
	// Assets
	AssetsGrubCfgFile = "assets/grub2/grub.cfg"

	AssetsGrubDefFileAzl3  = "assets/azurelinux/azurelinux-3.0/grub2/grub"
	AssetsGrubStubFileAzl3 = "assets/azurelinux/azurelinux-3.0/efi/grub/grub.cfg"

	AssetsGrubDefFileAzl4  = "assets/azurelinux/azurelinux-4.0/grub2/grub"
	AssetsGrubStubFileAzl4 = "assets/azurelinux/azurelinux-4.0/efi/grub/grub.cfg"

	// Verity Signature Module Files
	VerityMountBootPartitionSetupFile     = "verity-signature/90mountbootpartition/module-setup.sh"
	VerityMountBootPartitionGeneratorFile = "verity-signature/90mountbootpartition/mountbootpartition-generator.sh"
	VerityMountBootPartitionGenRulesFile  = "verity-signature/90mountbootpartition/mountbootpartition-genrules.sh"
	VerityMountBootPartitionScriptFile    = "verity-signature/90mountbootpartition/mountbootpartition.sh"

	// Verity Workaround Hook
	// Dracut cmdline hook that injects Upholds= directives on partition device
	// units to prevent a race condition where systemd-veritysetup@root.service
	// fails to start because its BindsTo= dependencies are consumed before the
	// relationship is established.
	VerityUpholdsWorkaroundFile = "verity-workaround/10-verity-upholds-workaround.sh"

	// Certificates
	MicrosoftSupplyChainRSARootCA2022File = "certificates/Microsoft Supply Chain RSA Root CA 2022.crt"
)

//go:embed assets verity-signature verity-workaround certificates
var ResourcesFS embed.FS
