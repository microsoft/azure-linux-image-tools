---
parent: Configuration
---

# selinux type

Added in v0.3.

## mode [string]

Specifies the mode to set SELinux to.

If this field is not specified, then the existing SELinux mode in the base image is
maintained.
Otherwise, the image is modified to match the requested SELinux mode.

The Image Customizer tool can enable SELinux on a base image with SELinux
disabled and it can disable SELinux on a base image that has SELinux enabled.
However, using a base image that already has the required SELinux mode will speed-up the
customization process.

If SELinux is enabled, then all the file-systems that support SELinux will have their
file labels updated/reset (using the `setfiles` command).

Supported options:

- `disabled`: Disables SELinux.

- `permissive`: Enables SELinux but only logs access rule violations.

- `enforcing`: Enables SELinux and enforces all the access rules.

- `force-enforcing`: Enables SELinux and sets it to enforcing in the kernel
  command-line.
  This means that SELinux can't be set to `permissive` using the `/etc/selinux/config`
  file.

Note: For images with SELinux enabled, the `selinux-policy` package must be installed.
This package contains the default SELinux rules and is required for SELinux-enabled
images to be functional.
The Image Customizer tool will report an error if the package is missing from
the image.

Note: If you wish to apply additional SELinux policies on top of the base SELinux
policy, then it is recommended to apply these new policies using a
([postCustomization](./scripts.md#postcustomization-script)) script.
After applying the policies, you do not need to call `setfiles` manually since it will
called automatically after the `postCustomization` scripts are run.

Example:

```yaml
os:
  selinux:
    mode: enforcing

  packages:
    install:
    # Required packages for SELinux.
    - selinux-policy
    - selinux-policy-modules
    
    # Optional packages that contain useful SELinux utilities.
    - setools-console
    - policycoreutils-python-utils
```

Added in v0.3.
