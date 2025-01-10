---
parent: Configuration
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

## uid [int]

The ID to use for the user.
This value is not used if the user already exists.

Valid range: 0-60000

Example:

```yaml
os:
  users:
  - name: test
    uid: 1000
```

## password [[password](./password.md)]

Specifies the user's password.

WARNING: Passwords should not be used in images used in production.

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

## primaryGroup [string]

The primary group of the user.

Example:

```yaml
os:
  users:
  - name: test
    primaryGroup: testgroup
```

## secondaryGroups [string[]]

Additional groups to assign to the user.

Example:

```yaml
os:
  users:
  - name: test
    secondaryGroups:
    - sudo
```

## startupCommand [string]

The command run when the user logs in.

Example:

```yaml
os:
  users:
  - name: test
    startupCommand: /sbin/nologin
```
