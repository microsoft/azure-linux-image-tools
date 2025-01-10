---
parent: Configuration
---

# mountPoint type

You can configure `mountPoint` in one of two ways:

1. **Structured Format**: Use `idType`, `options`, and `path` fields for a more detailed configuration.

   ```yaml
   mountPoint:
     path: /boot/efi
     options: umask=0077
     idType: part-uuid
   ```

2. **Shorthand Path Format**: Provide the mount path directly as a string when only `path` is required.

   ```yaml
   mountPoint: /boot/efi
   ```

   In this shorthand format, only the `path` is specified, and default values will be applied to any optional fields.

## idType [string]

Default: `part-uuid`

The partition ID type that should be used to recognize the partition on the disk.

Supported options:

- `uuid`: The filesystem's partition UUID.

- `part-uuid`: The partition UUID specified in the partition table.

- `part-label`: The partition label specified in the partition table.

## options [string]

The additional options used when mounting the file system.

These options are in the same format as
[mount](https://man7.org/linux/man-pages/man8/mount.8.html)'s
`-o` option (or the `fs_mntops` field of the
[fstab](https://man7.org/linux/man-pages/man5/fstab.5.html) file).

## path [string]

Required.

The absolute path of where the partition should be mounted.

The mounts will be sorted to ensure that parent directories are mounted before child
directories.
For example, `/boot` will be mounted before `/boot/efi`.
