// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package resources

import (
	"embed"
)

const (
	AssetsGrubCfgFile = "assets/grub2/grub.cfg"
	AssetsGrubDefFile = "assets/grub2/grub"

	// Verity Signature Module Files
	VerityMountBootPartitionSetupFile     = "verity-signature/90mountbootpartition/module-setup.sh"
	VerityMountBootPartitionGeneratorFile = "verity-signature/90mountbootpartition/mountbootpartition-generator.sh"
	VerityMountBootPartitionGenRulesFile  = "verity-signature/90mountbootpartition/mountbootpartition-genrules.sh"
	VerityMountBootPartitionScriptFile    = "verity-signature/90mountbootpartition/mountbootpartition.sh"
)

//go:embed assets verity-signature
var ResourcesFS embed.FS
