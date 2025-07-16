---
parent: Configuration
ancestor: Image Customizer
---

# user type

Options for configuring a user account.

## name [string]

Required.

The name of the user.

Example:

```yaml
os:
  users:
  - name: test
```

Added in v0.3.

## uid [int]

The ID to use for the user.

If the user already exists, providing this value will result in an error.

Valid range: 0-60000

Example:

```yaml
os:
  users:
  - name: test
    uid: 1000
```

Added in v0.3.

## password [[password](./password.md)]

Specifies the user's password.

WARNING: Passwords should not be used in images used in production.

Added in v0.3.

## passwordExpiresDays [int]

The number of days until the password expires and the user can no longer login.

Valid range: 0-99999. Set to -1 to remove expiry.

Example:

```yaml
os:
  users:
  - name: test
    passwordPath: test-password.txt
    passwordHashed: true
    passwordExpiresDays: 120
```

Added in v0.3.

## sshPublicKeyPaths [string[]]

A list of file paths to SSH public key files.
These public keys will be copied into the user's `~/.ssh/authorized_keys` file.

Note: It is preferable to use Microsoft Entra ID for SSH authentication, instead of
individual public keys.

Example:

```yaml
os:
  users:
  - name: test
    sshPublicKeyPaths:
    - id_ed25519.pub
```

Added in v0.3.

## sshPublicKeys [string[]]

A list of SSH public keys.
These public keys will be copied into the user's `~/.ssh/authorized_keys` file.

Note: It is preferable to use Microsoft Entra ID for SSH authentication, instead of
individual public keys.

Example:

```yaml
os:
  users:
  - name: test
    sshPublicKeys:
    - ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFyWtgGE06d/uBFQm70tYKvJKwJfRDoh06bWQQwC6Qkm test@test-machine
```

Added in v0.3.

## primaryGroup [string]

Set the primary group of the user.

If a value is provided, then the group must already exist.

If a value is not provided and the user does not exist, then a new group will be
automatically created with the same name as the user.

Example:

```yaml
os:
  users:
  - name: test
    primaryGroup: testgroup
```

Added in v0.3.

## secondaryGroups [string[]]

Additional groups to assign to the user. These groups must already exist.

Example:

```yaml
os:
  users:
  - name: test
    secondaryGroups:
    - sudo
```

Added in v0.3.

## startupCommand [string]

The command run when the user logs in.

Example:

```yaml
os:
  users:
  - name: test
    startupCommand: /sbin/nologin
```

Added in v0.3.

## homeDirectory [string]

Where the user's home directory should be located.

```yaml
os:
  users:
  - name: test
    homeDirectory: /var/home/test
```

Added in v0.5.
