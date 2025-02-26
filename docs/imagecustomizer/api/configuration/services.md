---
parent: Configuration
---

# services type

Options for configuring systemd services.

Added in v0.3.

## enable [string[]]

A list of services to enable.
That is, services that will be set to automatically run on OS boot.

Example:

```yaml
os:
  services:
    enable:
    - sshd
```

Added in v0.3.

## disable [string[]]

A list of services to disable.
That is, services that will be set to not automatically run on OS boot.

Example:

```yaml
os:
  services:
    disable:
    - sshd
```

Added in v0.3.
