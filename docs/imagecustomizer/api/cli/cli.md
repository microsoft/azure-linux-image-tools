---
title: Command line
parent: API
grand_parent: Image Customizer
nav_order: 1
---

# Image Customizer command line

## create

Creates a new Azure Linux image from scratch.

See [create subcommand](./create.md) for full documentation.

Added in v1.2.

## convert

Converts an image from one format to another without performing customization.

This is a streamlined command for simple format conversions, especially useful
when converting to COSI format.

See [convert subcommand](./convert.md) for full documentation.

Added in v1.2.

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
