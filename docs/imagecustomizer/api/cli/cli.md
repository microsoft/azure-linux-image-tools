---
title: Command line
parent: API
grand_parent: Image Customizer
nav_order: 1
---

# Image Customizer command line

## customize

Customizes a base OS image.

If no subcommand is specified, this subcommand is the default.

See [customize subcommand](./customize.md) for full documentation.

Added in v0.13. Prior to v0.13, there were no subcommands and `customize` was the only
operation supported.

## inject-files

Injects files into a disk image using an injection configuration.

See [inject-files subcommand](./inject-files.md) for full documentation.

Added in v0.14.

## --help

Displays the tool's quick help.

## --log-level=LEVEL

Default: `info`

The verbosity of logs the tool outputs.

Higher levels of logging may be useful for debugging what the tool is doing.

The levels from lowest to highest level of verbosity are: `panic`, `fatal`, `error`,
`warn`, `info`, `debug`, and `trace`.

Added in v0.3.

## --version

Prints the version of the tool.

## --preview-feature=FEATURE

Enable a preview feature via the command line without modifying the configuration file.

This option can be specified multiple times to enable multiple preview features.

Supported options: `base-configs`, `btrfs`, `cosi-compression`, `create`, `fedora-42`, `inject-files`,
`input-image-oci`, `kdump-boot-files`, `output-artifacts`, `output-selinux-policy`, `package-snapshot-time`,
`reinitialize-verity`, `ubuntu-22.04`, `ubuntu-24.04`, `uki`

Added in v1.2.
