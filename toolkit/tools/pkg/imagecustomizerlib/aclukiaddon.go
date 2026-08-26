// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"slices"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/grub"
)

var (
	ErrAclUkiAddonSplit = NewImageCustomizerError("AclUkiAddon:Split",
		"failed to split kernel command line into ACL UKI addons")
)

const (
	// aclCustomizedAddonName is the addon file Image Customizer uses to carry any kernel
	// command-line arguments that user configuration (e.g. os.uki.kernelCommandLine) adds or
	// changes relative to a kernel's existing addon structure. See
	// aclGetUkiAddonSpecsPreserving for why these arguments get their own dedicated file instead
	// of being folded into an existing native ACL addon file such as oem.addon.efi.
	aclCustomizedAddonName = "customized.addon.efi"
)

// aclVerityCmdlineArgNames lists every kernel argument name that identifies /usr dm-verity
// configuration (device locators, root hash digest, and options) rather than general OS
// configuration. Image Customizer never accepts a new or changed argument with one of these
// names from kernel command-line customization (see aclGetUkiAddonSpecsPreserving) -- this data
// is managed exclusively by Trident (per-A/B-slot device locators) and Image Customizer's own
// verity-hash refresh (aclUpdateVerityAddonTemplates / aclUpdateLiveVerityAddons; root hash
// digest), never by user cmdline configuration.
//
// This list was verified directly against a real ACL image's verity-a.addon.efi /
// verity-b.addon.efi / live verity.addon.efi content (branch-2 pipeline artifact): all three
// files carry exactly these four arguments, and nothing else. rd.systemd.verity is deliberately
// NOT included: it does not appear in any of those files (ACL's /usr dm-verity setup activates
// automatically off the presence of systemd.verity_usr_hash=, without needing an explicit enable
// flag), so treating it as verity-owned here would be speculative rather than evidence-based.
var aclVerityCmdlineArgNames = []string{
	"systemd.verity_usr_data",
	"systemd.verity_usr_hash",
	"systemd.verity_usr_options",
	"usrhash",
}

// aclParseCmdline tokenizes and parses cmdline into individual kernel command-line arguments,
// rejecting any argument whose value contains a variable expansion (e.g. $a) -- the parsed form of
// such an argument is truncated at the expansion, so it cannot be manipulated (split, deduplicated,
// or rewritten) faithfully.
func aclParseCmdline(cmdline string) ([]grubConfigLinuxArg, error) {
	tokens, err := grub.TokenizeConfig(cmdline)
	if err != nil {
		return nil, fmt.Errorf("failed to tokenize kernel command line:\n%w", err)
	}

	args, err := ParseCommandLineArgs(tokens)
	if err != nil {
		return nil, fmt.Errorf("failed to parse kernel command-line args:\n%w", err)
	}

	for _, arg := range args {
		if arg.ValueHasVarExpansion {
			return nil, fmt.Errorf("kernel command-line arg (%s) contains a variable expansion", arg.Arg)
		}
	}

	return args, nil
}

// argsToStrings returns the .Arg field of every element of args, in order.
func argsToStrings(args []grubConfigLinuxArg) []string {
	strs := make([]string, 0, len(args))
	for _, arg := range args {
		strs = append(strs, arg.Arg)
	}
	return strs
}

// removeArgByName returns args with every element whose Name equals name removed.
func removeArgByName(args []grubConfigLinuxArg, name string) []grubConfigLinuxArg {
	filtered := make([]grubConfigLinuxArg, 0, len(args))
	for _, arg := range args {
		if arg.Name == name {
			continue
		}
		filtered = append(filtered, arg)
	}
	return filtered
}

// aclGetUkiAddonSpecsPreserving returns ACL's UKI cmdline addon layout for a kernel, built to
// exactly mirror whatever addon structure the input image already has for that kernel: every
// existing addon file (oem.addon.efi, verity.addon.efi, firstboot.addon.efi, or any other file ACL
// or a future ACL version adds) passes through untouched, with only two narrow exceptions --
//
//  1. A new or changed kernel command-line argument that user configuration (e.g.
//     os.uki.kernelCommandLine) contributes is written to its own dedicated file,
//     aclCustomizedAddonName, instead of being folded into an existing native addon file. If that
//     argument's name collides with one already owned by a different existing addon file, the
//     stale copy is stripped out of that file (the new value lives only in aclCustomizedAddonName)
//     -- systemd-boot loads addons in a fixed filename-alphabetical order with last-value-wins
//     semantics, so a changed value cannot be reliably expressed by leaving stale copies in two
//     files at once.
//  2. Whatever cmdline the input image's main UKI carried directly in its own `.cmdline` PE
//     section (existingAddons[ukiMainCmdlineAddonKey]) is re-homed into its own addon file, named
//     ukiAddonFileName(kernel) (e.g. "vmlinuz-6.6.150.1-1.azl3.addon.efi"), since Image
//     Customizer's UKI build never re-embeds a cmdline into the rebuilt main UKI itself (see
//     buildMainUki) -- without this, whatever was directly embedded in main would be silently
//     dropped by any customization pass.
//
// aclCustomizedAddonName and ukiAddonFileName(kernel) are therefore the only two files Image
// Customizer is ever allowed to invent; if the input image has no addon structure at all for a
// kernel and no argument changes are needed, no addon files are produced for it either.
//
// A new or changed argument whose name is one of aclVerityCmdlineArgNames (see its doc comment) is
// always rejected with an error, regardless of whether that name previously existed anywhere: this
// data is never a supported target for kernel command-line customization.
//
// existingAddons is nil/empty when there's no known prior addon-file structure to preserve for
// this kernel at all (not even a main UKI cmdline) -- for ACL this should never actually happen in
// practice (every ACL kernel ships with existing UKI structure), but is handled the same way as
// any other case: with no existing files to account for or collide with, every argument in cmdline
// is either rejected (verity-named) or written to aclCustomizedAddonName.
func aclGetUkiAddonSpecsPreserving(kernel string, cmdline string, existingAddons map[string]string) ([]UkiAddonSpec, error) {
	cmdlineArgs, err := aclParseCmdline(cmdline)
	if err != nil {
		return nil, fmt.Errorf("%w:\n%w", ErrAclUkiAddonSplit, err)
	}

	// accountedFor holds every argument string (name=value) already present verbatim in some
	// existing file (including the input image's main UKI cmdline) -- these need no action, since
	// they're already covered by a passthrough entry in fileArgs below.
	accountedFor := map[string]bool{}

	// fileArgs holds the mutable, ordered argument list for every file that will be emitted as a
	// spec, keyed by its OUTPUT file name. The main UKI's own preserved cmdline
	// (existingAddons[ukiMainCmdlineAddonKey]) is keyed here under ukiAddonFileName(kernel) -- see
	// this function's doc comment for why.
	fileArgs := map[string][]grubConfigLinuxArg{}

	// argOwner maps an argument name to the OUTPUT file name (a key of fileArgs) that currently
	// owns it. Used to detect and resolve a collision when a new/changed argument from cmdline
	// needs to move into aclCustomizedAddonName.
	argOwner := map[string]string{}

	// customizedArgs/customizedArgIndexByName hold aclCustomizedAddonName's own final argument
	// list as it's built up below -- starting from its own pre-existing content (if any), then
	// receiving every new/changed/stripped argument.
	customizedArgs := []string(nil)
	customizedArgIndexByName := map[string]int{}

	fileNames := make([]string, 0, len(existingAddons))
	for fileName := range existingAddons {
		fileNames = append(fileNames, fileName)
	}
	slices.Sort(fileNames) // deterministic order regardless of Go's randomized map iteration order

	for _, fileName := range fileNames {
		addonCmdline := existingAddons[fileName]

		args, err := aclParseCmdline(addonCmdline)
		if err != nil {
			return nil, fmt.Errorf("%w: existing addon (%s):\n%w", ErrAclUkiAddonSplit, fileName, err)
		}
		for _, arg := range args {
			accountedFor[arg.Arg] = true
		}

		if fileName == aclCustomizedAddonName {
			// Defer emitting this as a spec until any new/updated arguments have been merged in
			// below, so the file is only written once with its final combined content.
			for _, arg := range args {
				customizedArgs = append(customizedArgs, arg.Arg)
				customizedArgIndexByName[arg.Name] = len(customizedArgs) - 1
			}
			continue
		}

		outputFileName := fileName
		if fileName == ukiMainCmdlineAddonKey {
			outputFileName = ukiAddonFileName(kernel)
		}

		fileArgs[outputFileName] = append(fileArgs[outputFileName], args...)
		for _, arg := range args {
			argOwner[arg.Name] = outputFileName
		}
	}

	for _, arg := range cmdlineArgs {
		if accountedFor[arg.Arg] {
			// Unchanged from what's already in some existing file; nothing to do.
			continue
		}

		if slices.Contains(aclVerityCmdlineArgNames, arg.Name) {
			return nil, fmt.Errorf(
				"%w: kernel command-line arg (%s) for kernel (%s) sets a dm-verity argument; "+
					"dm-verity configuration is managed exclusively by Trident and Image Customizer's "+
					"own verity addon refresh, and is never a supported target for kernel "+
					"command-line customization",
				ErrAclUkiAddonSplit, arg.Arg, kernel)
		}

		if owningFile, exists := argOwner[arg.Name]; exists {
			// Strip the stale/conflicting arg from the file that currently owns it; the new value
			// moves to aclCustomizedAddonName below. Systemd-boot's fixed alphabetical addon load
			// order can't guarantee an override in the owning file's favor, so the only safe way to
			// apply a changed value is to remove it from its old home entirely.
			fileArgs[owningFile] = removeArgByName(fileArgs[owningFile], arg.Name)
			delete(argOwner, arg.Name)
		}

		if idx, exists := customizedArgIndexByName[arg.Name]; exists {
			// Updates a value this kernel was previously customized with; replace in place.
			customizedArgs[idx] = arg.Arg
			continue
		}

		customizedArgs = append(customizedArgs, arg.Arg)
		customizedArgIndexByName[arg.Name] = len(customizedArgs) - 1
	}

	specs := []UkiAddonSpec(nil)
	for fileName, args := range fileArgs {
		if len(args) == 0 {
			// Every argument that used to be in this file was stripped out by a collision above;
			// omit it entirely rather than emit an empty addon.
			continue
		}
		specs = append(specs, UkiAddonSpec{FileName: fileName, Cmdline: GrubArgsToString(argsToStrings(args))})
	}

	if len(customizedArgs) > 0 {
		specs = append(specs, UkiAddonSpec{FileName: aclCustomizedAddonName, Cmdline: GrubArgsToString(customizedArgs)})
	}

	slices.SortFunc(specs, func(a, b UkiAddonSpec) int { return strings.Compare(a.FileName, b.FileName) })

	return specs, nil
}
