// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package resources

import (
	"embed"
)

const (
	// Assets
	AssetsGrubCfgFile      = "assets/grub2/grub.cfg"
	AssetsGrubDefFileAzl3  = "assets/grub2/grub-azl3"
	AssetsGrubDefFileAzl4  = "assets/grub2/grub-azl4"
	AssetsGrubStubFileAzl3 = "assets/efi/grub/grub-azl3.cfg"
	AssetsGrubStubFileAzl4 = "assets/efi/grub/grub-azl4.cfg"

	// Verity Signature Module Files
	VerityMountBootPartitionSetupFile     = "verity-signature/90mountbootpartition/module-setup.sh"
	VerityMountBootPartitionGeneratorFile = "verity-signature/90mountbootpartition/mountbootpartition-generator.sh"
	VerityMountBootPartitionGenRulesFile  = "verity-signature/90mountbootpartition/mountbootpartition-genrules.sh"
	VerityMountBootPartitionScriptFile    = "verity-signature/90mountbootpartition/mountbootpartition.sh"

	// Certificates
	MicrosoftSupplyChainRSARootCA2022File = "certificates/Microsoft Supply Chain RSA Root CA 2022.crt"
)

//go:embed assets verity-signature certificates
var ResourcesFS embed.FS
