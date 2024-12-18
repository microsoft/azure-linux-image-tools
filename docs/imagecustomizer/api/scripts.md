# scripts

Specifies custom scripts to run during the customization process.

Note: Script files must be in the same directory or a child directory of the directory
that contains the config file.

- [scripts](#scripts)
  - [script type](#script-type)
    - [path \[string\]](#path-string)
    - [content \[string\]](#content-string)
    - [interpreter \[string\]](#interpreter-string)
    - [arguments \[string\[\]\]](#arguments-string)
    - [environmentVariables \[map\<string, string\>\]](#environmentvariables-mapstring-string)
    - [name \[string\]](#name-string)
  - [postCustomization \[script\[\]\]](#postcustomization-script)
  - [finalizeCustomization \[script\[\]\]](#finalizecustomization-script)

## script type

Points to a script file (typically a Bash script) to be run during customization.

Scripts are run with a limited set of
[capabilities](https://man7.org/linux/man-pages/man7/capabilities.7.html). Specifically:

- `CAP_CHOWN`
- `CAP_DAC_OVERRIDE`
- `CAP_DAC_READ_SEARCH`
- `CAP_FOWNER`
- `CAP_SETFCAP`

Restricting the set of capabilities helps prevent scripts from accidentally affecting
the host kernel.

WARNING: Custom scripts are not considered to be on security boundary.
Only use config files that you trust (or run image customizer in a security sandbox).

<div id="script-path"></div>

### path [string]

The path of the script.

This must be in the same directory or a sub-directory that the config file is located
in.

Only one of `path` or `content` may be specified.

Example:

```yaml
scripts:
  postCustomization:
  - path: scripts/a.sh
```

### content [string]

The contents of the script to run.

The script is written to a temporary file under the customized OS's `/tmp` directory.

Only one of `path` or `content` may be specified.

Example:

```yaml
scripts:
  postCustomization:
  - content: |
      echo "Hello, World"
```

### interpreter [string]

The program to run the script with.

If not specified, then the script is run by `/bin/sh`.

Example:

```yaml
scripts:
  postCustomization:
  - content: |
      print("Hello, World")
    interpreter: python3
```

### arguments [string[]]

Additional arguments to pass to the script.

Example:

```yaml
scripts:
  postCustomization:
  - path: scripts/a.sh
    arguments:
    - abc
```

### environmentVariables [map\<string, string>]

Additional environment variables to set on the program.

Example:

```yaml
scripts:
  postCustomization:
  - content: |
      echo "$a $b"
    environmentVariables:
      a: hello
      b: world
```

<div id="script-name"></div>

### name [string]

The name of the script.

This field is only used to refer to the script in the logs.
It is particularly useful when `content` is used.

Example:

```yaml
scripts:
  postCustomization:
  - content: |
      echo "Hello, World"
    name: greetings
```

## postCustomization [[script](#script-type)[]]

Scripts to run after all the in-built customization steps have run.

These scripts are run under a chroot of the customized OS.

Example:

```yaml
scripts:
  postCustomization:
  - path: scripts/a.sh
```

## finalizeCustomization [[script](#script-type)[]]

Scripts to run at the end of the customization process.

In particular, these scripts run after:

1. The `setfiles` command has been called to update/fix the SELinux files labels (if
   SELinux is enabled), and

2. The temporary `/etc/resolv.conf` file has been deleted,

but before the conversion to the requested output type.
(See, [Operation ordering](./configuration.md#operation-ordering) for details.)

Most scripts should be added to [postCustomization](./scripts.md#postcustomization-script).
Only add scripts to [finalizeCustomization](./scripts.md#finalizecustomization-script) if you want
to customize the `/etc/resolv.conf` file or you want manually set SELinux file labels.

These scripts are run under a chroot of the customized OS.

Example:

```yaml
scripts:
  finalizeCustomization:
  - path: scripts/b.sh
```
