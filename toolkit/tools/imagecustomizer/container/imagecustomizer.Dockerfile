# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

ARG BASE_IMAGE="mcr.microsoft.com/azurelinux/base/core:3.0"

FROM ${BASE_IMAGE}

ARG AZ_MON_CONN_STR

ENV AZURE_MONITOR_CONNECTION_STRING=${AZ_MON_CONN_STR}

RUN tdnf update -y && \
   tdnf install -y qemu-img rpm coreutils util-linux systemd systemd-udev openssl \
      sed createrepo_c squashfs-tools cdrkit parted e2fsprogs dosfstools \
      xfsprogs btrfs-progs zstd veritysetup grub2 grub2-pc systemd-ukify binutils lsof \
      tar pigz && \
   tdnf clean all

# Copy all necessary files
COPY usr /usr

ENTRYPOINT ["/usr/lib/imagecustomizer/entrypoint.sh"]
