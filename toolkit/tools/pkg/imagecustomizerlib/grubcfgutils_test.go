// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/stretchr/testify/assert"
)

// selinuxArgState describes the state of a single SELinux-related kernel command-line arg.
// If Present is false, the arg is not on the cmdline at all (Value is ignored).
type selinuxArgState struct {
	Present bool
	Value   string
}

// TestGetSELinuxModeFromLinuxArgs_AllCombinations exhaustively covers the 3x3x3 = 27 combinations of
// (security, selinux, enforcing) kernel command-line arg states.
func TestGetSELinuxModeFromLinuxArgs_AllCombinations(t *testing.T) {
	const (
		Disabled       = imagecustomizerapi.SELinuxModeDisabled
		Default        = imagecustomizerapi.SELinuxModeDefault
		ForceEnforcing = imagecustomizerapi.SELinuxModeForceEnforcing
	)

	absent := selinuxArgState{Present: false}
	present := func(v string) selinuxArgState { return selinuxArgState{Present: true, Value: v} }

	testCases := []struct {
		Name          string
		Security      selinuxArgState
		Selinux       selinuxArgState
		Enforcing     selinuxArgState
		ExpectedBase  imagecustomizerapi.SELinuxMode
		ExpectedDefer imagecustomizerapi.SELinuxMode
	}{
		// security=absent
		{"security=absent/selinux=absent/enforcing=absent", absent, absent, absent, Disabled, Default},
		{"security=absent/selinux=absent/enforcing=0", absent, absent, present("0"), Disabled, Disabled},
		{"security=absent/selinux=absent/enforcing=1", absent, absent, present("1"), Disabled, Disabled},
		{"security=absent/selinux=0/enforcing=absent", absent, present("0"), absent, Disabled, Disabled},
		{"security=absent/selinux=0/enforcing=0", absent, present("0"), present("0"), Disabled, Disabled},
		{"security=absent/selinux=0/enforcing=1", absent, present("0"), present("1"), Disabled, Disabled},
		{"security=absent/selinux=1/enforcing=absent", absent, present("1"), absent, Disabled, Disabled},
		{"security=absent/selinux=1/enforcing=0", absent, present("1"), present("0"), Disabled, Disabled},
		{"security=absent/selinux=1/enforcing=1", absent, present("1"), present("1"), Disabled, Disabled},

		// security=selinux
		{"security=selinux/selinux=absent/enforcing=absent", present("selinux"), absent, absent, Disabled, Disabled},
		{"security=selinux/selinux=absent/enforcing=0", present("selinux"), absent, present("0"), Disabled, Disabled},
		{"security=selinux/selinux=absent/enforcing=1", present("selinux"), absent, present("1"), Disabled, Disabled},
		{"security=selinux/selinux=0/enforcing=absent", present("selinux"), present("0"), absent, Disabled, Disabled},
		{"security=selinux/selinux=0/enforcing=0", present("selinux"), present("0"), present("0"), Disabled, Disabled},
		{"security=selinux/selinux=0/enforcing=1", present("selinux"), present("0"), present("1"), Disabled, Disabled},
		{"security=selinux/selinux=1/enforcing=absent", present("selinux"), present("1"), absent, Default, Default},
		{"security=selinux/selinux=1/enforcing=0", present("selinux"), present("1"), present("0"), Default, Default},
		{"security=selinux/selinux=1/enforcing=1", present("selinux"), present("1"), present("1"), ForceEnforcing, ForceEnforcing},

		// security=apparmor
		{"security=apparmor/selinux=absent/enforcing=absent", present("apparmor"), absent, absent, Disabled, Disabled},
		{"security=apparmor/selinux=absent/enforcing=0", present("apparmor"), absent, present("0"), Disabled, Disabled},
		{"security=apparmor/selinux=absent/enforcing=1", present("apparmor"), absent, present("1"), Disabled, Disabled},
		{"security=apparmor/selinux=0/enforcing=absent", present("apparmor"), present("0"), absent, Disabled, Disabled},
		{"security=apparmor/selinux=0/enforcing=0", present("apparmor"), present("0"), present("0"), Disabled, Disabled},
		{"security=apparmor/selinux=0/enforcing=1", present("apparmor"), present("0"), present("1"), Disabled, Disabled},
		{"security=apparmor/selinux=1/enforcing=absent", present("apparmor"), present("1"), absent, Disabled, Disabled},
		{"security=apparmor/selinux=1/enforcing=0", present("apparmor"), present("1"), present("0"), Disabled, Disabled},
		{"security=apparmor/selinux=1/enforcing=1", present("apparmor"), present("1"), present("1"), Disabled, Disabled},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			args := buildSELinuxArgs(tc.Security, tc.Selinux, tc.Enforcing)

			gotBase, err := getSELinuxModeFromLinuxArgs(args)
			assert.NoError(t, err, "getSELinuxModeFromLinuxArgs returned unexpected error")
			assert.Equal(t, tc.ExpectedBase, gotBase, "getSELinuxModeFromLinuxArgs mismatch")

			gotDefer, err := getSELinuxModeFromLinuxArgsDeferIfMissing(args)
			assert.NoError(t, err, "getSELinuxModeFromLinuxArgsDeferIfMissing returned unexpected error")
			assert.Equal(t, tc.ExpectedDefer, gotDefer, "getSELinuxModeFromLinuxArgsDeferIfMissing mismatch")
		})
	}
}

// buildSELinuxArgs constructs a synthetic []grubConfigLinuxArg that only populates the Name and Value fields
// (which are the only fields read by getSELinuxModeFromLinuxArgs / findKernelCommandLineArgValue /
// findMatchingCommandLineArgs). This avoids needing to round-trip through the grub tokenizer for unit tests.
func buildSELinuxArgs(security, selinux, enforcing selinuxArgState) []grubConfigLinuxArg {
	var args []grubConfigLinuxArg
	if security.Present {
		args = append(args, grubConfigLinuxArg{
			Name:  "security",
			Value: security.Value,
			Arg:   fmt.Sprintf("security=%s", security.Value),
		})
	}
	if selinux.Present {
		args = append(args, grubConfigLinuxArg{
			Name:  "selinux",
			Value: selinux.Value,
			Arg:   fmt.Sprintf("selinux=%s", selinux.Value),
		})
	}
	if enforcing.Present {
		args = append(args, grubConfigLinuxArg{
			Name:  "enforcing",
			Value: enforcing.Value,
			Arg:   fmt.Sprintf("enforcing=%s", enforcing.Value),
		})
	}
	return args
}
