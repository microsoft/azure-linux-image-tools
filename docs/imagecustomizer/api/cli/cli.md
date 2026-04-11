---
title: Command line
parent: API
grand_parent: Image Customizer
nav_order: 1
---

# Image Customizer command line

## create

Creates a new Linux image from scratch.

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

## validate-config

Validates a configuration file without running the actual customization process.

See [validate-config subcommand](./validate-config.md) for full documentation.

Added in v1.2.

## --help

Displays the tool's quick help.

## --log-level=LEVEL

Default: `info`

The verbosity of logs the tool outputs.

Higher levels of logging may be useful for debugging what the tool is doing.

The levels from lowest to highest level of verbosity are: `panic`, `fatal`, `error`,
`warn`, `info`, `debug`, and `trace`.

Added in v0.3.

## --log-format=FORMAT

Sets the format of the logs.

Options:

- `text`: Output the logs as a human readable text log.
- `json`: Output the logs as a sequence of JSON objects.

Default option: `text`

There are no backwards compatibility guarantees for the `text` log format. Both the
format of the logs and the contents of the logs can be changed between releases.

For the `json` log format, the only guarantee is that the logs are outputted as a
sequence of JSON objects. The actual contents of the log messages can change between
releases.

Added in v1.3.

## --version

Prints the version of the tool.
