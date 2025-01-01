# password type

Specifies a password for a user.

WARNING: Passwords should not be used in images used in production.

This feature is intended for debugging purposes only.
As such, this feature has been disabled in official builds of the Image Customizer tool.

Instead of using passwords, you should use an authentication system that relies on
cryptographic keys.
For example, SSH with Microsoft Entra ID authentication.

Example:

```yaml
os:
  users:
  - name: test
    password:
      type: locked
```

## type [string]

The manner in which the password is provided.

Supported options:

- `locked`: Password login is disabled for the user. This is the default behavior.

Options for debugging purposes only (disabled by default):

- `plain-text`: The value is a plain-text password.

- `hashed`: The value is a password that has been pre-hashed.
  (For example, by using `openssl passwd`.)

- `plain-text-file`: The value is a path to a file containing a plain-text password.

- `hashed-file`: The value is a path to a file containing a pre-hashed password.

## value [string]

The password's value.
The meaning of this value depends on the type property.
