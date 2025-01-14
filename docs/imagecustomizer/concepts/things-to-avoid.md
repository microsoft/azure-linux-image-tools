---
parent: Concepts
nav_order: 4
---

# Things to avoid

The Image Customizer tool provides the option to run custom scripts as part of the
customization process.
These can be used to handle scenarios not covered by the Image Customizer tool.
However, these scripts are only run within a chroot environment, which while it is kind
of similar to containers, is very explicitly not a sandbox environment.
So, such scripts have the ability to modify the host build system.

In particular, you should be very wary of commands that have the ability to change the
runtime kernel settings.
And even commands that only read runtime kernel settings are probably doing the wrong
thing, since the host build system's kernel is likely entirely unrelated to the
customized OS's kernel.

Examples of commands to avoid:

- `ip`
- `iptables`
- `iptables-save`
- `ip6tables-save`
- `modprobe`
- `sysctl`

Instead, you should you make use of config files that set the runtime kernel settings
during OS boot.

Example config directories to use instead:

- `/etc/sysctl.d` (`systemd-sysctl.service`)
