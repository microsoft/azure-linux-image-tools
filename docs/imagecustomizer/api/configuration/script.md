# script type

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

## path [string]

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

## content [string]

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

## interpreter [string]

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

## arguments [string[]]

Additional arguments to pass to the script.

Example:

```yaml
scripts:
  postCustomization:
  - path: scripts/a.sh
    arguments:
    - abc
```

## environmentVariables [map\<string, string>]

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

## name [string]

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
