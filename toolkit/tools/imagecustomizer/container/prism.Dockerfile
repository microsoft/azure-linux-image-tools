# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

FROM mcr.microsoft.com/azurelinux/base/core:3.0
RUN tdnf update -y && \
   tdnf install -y qemu-img rpm coreutils util-linux systemd openssl \
      sed createrepo_c squashfs-tools cdrkit parted e2fsprogs dosfstools \
      xfsprogs zstd veritysetup grub2 grub2-pc systemd-ukify binutils lsof

COPY . /
