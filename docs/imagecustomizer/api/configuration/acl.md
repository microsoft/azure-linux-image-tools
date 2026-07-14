---
parent: Configuration
ancestor: Image Customizer
---

# acl type

{: .warning }
> This is a narrow, **Azure Container Linux (ACL) only** API. It is only valid when the target
> image is an ACL image; it is rejected for all other distros. Each capability is gated behind its
> own preview feature (`acl-grow-partitions` for `usr`/`esp`, `acl-oem-id` for `oemId`).

Narrow, ACL-only configuration for Azure Container Linux images. It can grow ACL's fixed,
well-known standard partitions to explicit target sizes and override the OEM id on the boot kernel
command line.

ACL images ship with a sealed, dm-verity-protected partition layout whose `/usr` is too small for
some SKUs. The partition-grow API enlarges specific, pre-existing ACL partitions in place. It
**only ever grows** them — it never reorders, adds, removes, or reformats partitions, and it
preserves every partition's PARTUUID, type GUID, label, and GPT attribute bits (including the
systemd A/B bits) exactly.

Rules:

- **Grow-only.** Requesting a size smaller than the current partition size is an error. Requesting
  the current size is a no-op.
- **ACL only.** Using `acl` on a non-ACL image is an error.
- Growing `usr` re-seals `/usr` verity at the new size and rebuilds the UKI. This requires
  [`storage.reinitializeVerity: all`](./storage.md#reinitializeverity-string) (and the
  `reinitialize-verity` preview feature).

Example:

```yaml
previewFeatures:
- acl-grow-partitions
- acl-oem-id
- reinitialize-verity
- preview-distro-version

storage:
  reinitializeVerity: all

acl:
  usr:
    size: 2G
  oemId: metal
```

Added in v1.6.

## usr [[aclPartitionGrow](#aclpartitiongrow-type)]

Optional. Requires the `acl-grow-partitions` preview feature.

Grows ACL's `/usr` partitions. ACL uses an A/B `/usr` (`USR-A` and `USR-B`); both partitions are
grown to the same size to keep the A/B pair symmetric. Only the active `/usr` filesystem (btrfs) is
resized and re-sealed; the A/B second copy keeps its original, self-consistent verity seal.

Added in v1.6.

## esp [[aclPartitionGrow](#aclpartitiongrow-type)]

Optional. Requires the `acl-grow-partitions` preview feature.

Grows ACL's EFI system partition (`EFI-SYSTEM`, mounted at `/boot` on ACL). The vfat filesystem is
recreated at the larger size, preserving its volume id, label, and files.

Added in v1.6.

## oemId [string]

Optional. Requires the `acl-oem-id` preview feature.

Overrides the flatcar OEM id (`flatcar.oem.id`) on the boot kernel command line of every
regenerated UKI. ACL's base image carries `flatcar.oem.id=azure`; on other SKUs (e.g. bare-metal)
this must be changed so the correct platform provider is selected and OEM-specific units (which
match on the presence of the old id) do not activate.

When set, all existing `flatcar.oem.id=*` and `coreos.oem.id=*` tokens are removed from the kernel
command line and `flatcar.oem.id=<value>` is set exactly once.

Must be lowercase alphanumeric (e.g. `metal`, `azure`, `qemu`, `gce`).

Added in v1.6.

# aclPartitionGrow type

## size [string]

Required.

The target size to grow the partition to.

Format: `<NUM>(K|M|G|T)` — an explicit size in KiB (`K`), MiB (`M`), GiB (`G`), or TiB (`T`). Must
be a multiple of 1 MiB.

Must be greater than or equal to the partition's current size (grow-only).

Added in v1.6.
