// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
	"regexp"
)

// Acl holds the narrow, ACL-only configuration for Azure Container Linux images.
//
// This is an internal, preview-gated API. It is only valid for ACL target images. It currently
// supports growing ACL's existing, sealed standard partitions to explicit target sizes (it only
// ever enlarges them; never reorders, adds, removes, or reformats them) and overriding the OEM id
// baked into the boot kernel command line.
type Acl struct {
	// Usr grows ACL's /usr partition(s) (USR-A and USR-B, kept the same size).
	Usr *AclPartitionGrow `yaml:"usr" json:"usr,omitempty"`
	// Esp grows ACL's EFI system partition (mounted at /boot on ACL).
	Esp *AclPartitionGrow `yaml:"esp" json:"esp,omitempty"`
	// OemId overrides the flatcar OEM id (flatcar.oem.id) on the boot kernel command line. ACL's
	// base UKI carries flatcar.oem.id=azure; on other SKUs (e.g. bare-metal) this must be changed
	// (e.g. to "metal") so the correct platform provider is used and azure-specific units do not
	// activate. Example values: metal, azure, qemu, gce.
	OemId string `yaml:"oemId" json:"oemId,omitempty"`
}

// AclPartitionGrow describes the desired grown size of an ACL standard partition.
type AclPartitionGrow struct {
	// Size is the target size to grow the partition to. Growing only: requesting a size smaller
	// than the current partition size is an error; requesting the current size is a no-op.
	Size DiskSize `yaml:"size" json:"size,omitempty"`
}

// aclOemIdRegex matches valid flatcar OEM ids: non-empty, lowercase alphanumeric (e.g. metal,
// azure, qemu, gce, ec2).
var aclOemIdRegex = regexp.MustCompile(`^[a-z0-9]+$`)

func (a *Acl) IsValid() error {
	if a.Usr == nil && a.Esp == nil && a.OemId == "" {
		return fmt.Errorf("'acl' must specify at least one of 'usr', 'esp', or 'oemId'")
	}

	if a.Usr != nil {
		if err := a.Usr.IsValid(); err != nil {
			return fmt.Errorf("invalid 'acl.usr':\n%w", err)
		}
	}

	if a.Esp != nil {
		if err := a.Esp.IsValid(); err != nil {
			return fmt.Errorf("invalid 'acl.esp':\n%w", err)
		}
	}

	if a.OemId != "" && !aclOemIdRegex.MatchString(a.OemId) {
		return fmt.Errorf("invalid 'acl.oemId' (%s): must be lowercase alphanumeric", a.OemId)
	}

	return nil
}

func (p *AclPartitionGrow) IsValid() error {
	if p.Size <= 0 {
		return fmt.Errorf("'size' must be specified and greater than 0")
	}

	return p.Size.IsValid()
}
