---
parent: Configuration
ancestor: Image Customizer
---

# acl type

{: .warning }
> This is a narrow, **Azure Container Linux (ACL) only** API and is gated behind the
> `acl-grow-partitions` preview feature. It is only valid when the target image is an ACL image;
> it is rejected for all other distros.

Grows ACL's fixed, well-known standard partitions to explicit target sizes.

ACL images ship with a sealed, dm-verity-protected partition layout whose `/usr` is too small for
some SKUs. This API enlarges specific, pre-existing ACL partitions in place. It **only ever grows**
them — it never reorders, adds, removes, or reformats partitions, and it preserves every
partition's PARTUUID, type GUID, label, and GPT attribute bits (including the systemd A/B bits)
exactly.

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
- reinitialize-verity
- preview-distro-version

storage:
  reinitializeVerity: all

acl:
  usr:
    size: 2G
```

Added in v1.6.

## usr [[aclPartitionGrow](#aclpartitiongrow-type)]

Optional.

Grows ACL's `/usr` partitions. ACL uses an A/B `/usr` (`USR-A` and `USR-B`); both partitions are
grown to the same size to keep the A/B pair symmetric. Only the active `/usr` filesystem (btrfs) is
resized and re-sealed; the A/B second copy keeps its original, self-consistent verity seal.

Added in v1.6.

## esp [[aclPartitionGrow](#aclpartitiongrow-type)]

Optional.

Grows ACL's EFI system partition (`EFI-SYSTEM`, mounted at `/boot` on ACL). The vfat filesystem is
recreated at the larger size, preserving its volume id, label, and files.

Added in v1.6.

# aclPartitionGrow type

## size [string]

Required.

The target size to grow the partition to.

Format: `<NUM>(K|M|G|T)` — an explicit size in KiB (`K`), MiB (`M`), GiB (`G`), or TiB (`T`). Must
be a multiple of 1 MiB.

Must be greater than or equal to the partition's current size (grow-only).

Added in v1.6.
