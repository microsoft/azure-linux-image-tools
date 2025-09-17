---
parent: Concepts
nav_order: 6
sidebar_position: 1
---

# Verity Image Recommendations

The Verity-enabled root filesystem is always mounted as read-only. Its root hash
and hash tree are computed at build time and verified by systemd during the
initramfs phase on each boot. When enabling the Verity feature on the root filesystem,
it is recommended to create a writable persistent partition for any directories that
require write access. Critical files and directories can be redirected to the
writable partition using symlinks or similar methods.

Please also note that some services and programs on Azure Linux may require
specific handling when using Verity. Depending on user needs, there are
different configuration options that offer tradeoffs between convenience and
security. Some configurations can be made flexible to allow changes, while
others may be set as immutable for enhanced security.
