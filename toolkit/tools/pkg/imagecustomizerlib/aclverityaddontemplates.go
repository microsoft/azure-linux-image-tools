// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/grub"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
)

// aclVerityAddonTemplatesDir is the directory on the image ESP where ACL stores its per-A/B-slot
// verity addon templates (verity-a.addon.efi / verity-b.addon.efi). Trident copies whichever
// template matches the target A/B slot into the live UKI's own addon directory as
// verity.addon.efi during every clean install and A/B update (see
// activate_verity_addon_for_target_volume in trident's crates/trident/src/engine/boot/uki.rs).
//
// These templates are baked in by acl-scripts at base-image build time and live outside the UKI's
// own `.efi.extra.d/` addon directory, so ImageCustomizer's normal addon-splitting logic
// (aclGetUkiAddonSpecs / aclGetUkiAddonSpecsPreserving) never sees or updates them.
//
// If ImageCustomizer customizes /usr (changing its dm-verity root hash) without also refreshing
// these two template files, they keep carrying the pre-customization hash. Trident's later
// per-slot activation would then copy that stale hash into the live verity.addon.efi, causing a
// dm-verity mismatch (and boot failure) the next time that slot is selected.
const aclVerityAddonTemplatesDir = "acl/uki-addons"

// aclVerityAddonTemplateFileNames lists the two per-slot verity addon templates Trident expects,
// matching activate_verity_addon_for_target_volume's slot-A / slot-B file names exactly.
var aclVerityAddonTemplateFileNames = []string{"verity-a.addon.efi", "verity-b.addon.efi"}

// aclVerityHashArgNames lists the kernel argument names that carry the /usr dm-verity root hash
// digest. Both are kept in sync, since they encode the same digest in two forms. Other verity args
// that may appear in these templates (e.g. systemd.verity_usr_data=, systemd.verity_usr_options=)
// encode per-slot device addressing, not content, and must NOT be rewritten here.
var aclVerityHashArgNames = []string{"usrhash", "systemd.verity_usr_hash"}

// extractAclVerityUsrHash returns the /usr dm-verity root hash digest embedded in cmdline (the
// value of usrhash= or systemd.verity_usr_hash=, whichever is present), and whether one was found.
func extractAclVerityUsrHash(cmdline string) (string, bool, error) {
	tokens, err := grub.TokenizeConfig(cmdline)
	if err != nil {
		return "", false, fmt.Errorf("failed to tokenize kernel command line:\n%w", err)
	}

	args, err := ParseCommandLineArgs(tokens)
	if err != nil {
		return "", false, fmt.Errorf("failed to parse kernel command-line args:\n%w", err)
	}

	for _, arg := range args {
		if slices.Contains(aclVerityHashArgNames, arg.Name) && arg.Value != "" {
			return arg.Value, true, nil
		}
	}

	return "", false, nil
}

// aclRewriteVerityHashArgs replaces the value of every usrhash= / systemd.verity_usr_hash=
// argument in cmdline with newHash, leaving every other argument (including other verity args such
// as systemd.verity_usr_data= / systemd.verity_usr_options=) untouched. Returns the rewritten
// cmdline and whether any argument was actually changed.
func aclRewriteVerityHashArgs(cmdline string, newHash string) (string, bool, error) {
	tokens, err := grub.TokenizeConfig(cmdline)
	if err != nil {
		return "", false, fmt.Errorf("failed to tokenize kernel command line:\n%w", err)
	}

	args, err := ParseCommandLineArgs(tokens)
	if err != nil {
		return "", false, fmt.Errorf("failed to parse kernel command-line args:\n%w", err)
	}

	changed := false
	rewrittenArgs := make([]string, 0, len(args))
	for _, arg := range args {
		if arg.ValueHasVarExpansion {
			// The parsed form of an arg with a variable expansion is truncated at the expansion, so
			// the arg cannot be rewritten faithfully.
			return "", false, fmt.Errorf("kernel command-line arg (%s) contains a variable expansion", arg.Arg)
		}

		if slices.Contains(aclVerityHashArgNames, arg.Name) {
			if arg.Value != newHash {
				changed = true
			}
			rewrittenArgs = append(rewrittenArgs, fmt.Sprintf("%s=%s", arg.Name, newHash))
			continue
		}

		rewrittenArgs = append(rewrittenArgs, arg.Arg)
	}

	return GrubArgsToString(rewrittenArgs), changed, nil
}

// rebuildAddonEfiAtPath rebuilds the UKI addon PE file at addonPath (overwriting it in place) with
// cmdline as its `.cmdline` section, using ukify with the given addon stub.
func rebuildAddonEfiAtPath(addonPath string, cmdline string, stubPath string) error {
	ukifyCmd := []string{
		"build",
		fmt.Sprintf("--cmdline=%s", cmdline),
		fmt.Sprintf("--stub=%s", stubPath),
		fmt.Sprintf("--output=%s", addonPath),
	}

	err := shell.ExecuteLiveWithErr(1, "ukify", ukifyCmd...)
	if err != nil {
		return fmt.Errorf("failed to rebuild UKI addon (%s):\n%w", addonPath, err)
	}

	return nil
}

// aclUpdateVerityAddonTemplates refreshes the /usr dm-verity root hash embedded in ACL's per-slot
// verity addon templates (acl/uki-addons/verity-a.addon.efi and verity-b.addon.efi on the image
// ESP), if present, so they match newUsrHash.
//
// This is a silent no-op if:
//   - newUsrHash is empty (the image has no /usr dm-verity root hash to propagate), or
//   - the template directory does not exist (non-ACL image, or an older ACL image that predates
//     acl-scripts' per-slot verity addon templates).
//
// A missing individual template file (only one of the two present) is logged and skipped rather
// than treated as an error, since ImageCustomizer's job here is a best-effort refresh, not template
// creation -- if a template is genuinely required at deploy time, Trident's own activation step
// already enforces that and fails loudly there.
func aclUpdateVerityAddonTemplates(espMountDir string, buildDir string, addonStubPath string, newUsrHash string) error {
	if newUsrHash == "" {
		logger.Log.Debugf("No /usr dm-verity root hash to propagate; skipping ACL verity addon template refresh")
		return nil
	}

	templateDir := filepath.Join(espMountDir, aclVerityAddonTemplatesDir)
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		logger.Log.Debugf("No ACL verity addon template directory at (%s); skipping refresh", templateDir)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to stat ACL verity addon template directory (%s):\n%w", templateDir, err)
	}

	for _, fileName := range aclVerityAddonTemplateFileNames {
		templatePath := filepath.Join(templateDir, fileName)

		if _, err := os.Stat(templatePath); os.IsNotExist(err) {
			logger.Log.Warnf("ACL verity addon template (%s) not found; skipping refresh", templatePath)
			continue
		} else if err != nil {
			return fmt.Errorf("failed to stat ACL verity addon template (%s):\n%w", templatePath, err)
		}

		cmdline, err := extractCmdlineFromSinglePE(templatePath, buildDir)
		if err != nil {
			return fmt.Errorf("failed to extract cmdline from ACL verity addon template (%s):\n%w", templatePath, err)
		}

		newCmdline, changed, err := aclRewriteVerityHashArgs(cmdline, newUsrHash)
		if err != nil {
			return fmt.Errorf("failed to rewrite verity hash args in ACL verity addon template (%s):\n%w", templatePath, err)
		}

		if !changed {
			logger.Log.Debugf("ACL verity addon template (%s) already has the current /usr verity root hash", templatePath)
			continue
		}

		err = rebuildAddonEfiAtPath(templatePath, newCmdline, addonStubPath)
		if err != nil {
			return fmt.Errorf("failed to refresh ACL verity addon template (%s):\n%w", templatePath, err)
		}

		logger.Log.Infof("Refreshed /usr dm-verity root hash in ACL verity addon template (%s)", templatePath)
	}

	return nil
}
