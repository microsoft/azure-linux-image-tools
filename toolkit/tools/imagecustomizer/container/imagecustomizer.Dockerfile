# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

ARG BASE_IMAGE="mcr.microsoft.com/azurelinux/base/core:3.0"

FROM ${BASE_IMAGE}

ARG AZ_MON_CONN_STR

ENV AZURE_MONITOR_CONNECTION_STRING=${AZ_MON_CONN_STR}

RUN tdnf update -y && \
   tdnf install -y azurelinux-repos-cloud-native && \
   tdnf update -y && \
   tdnf install -y qemu-img rpm coreutils util-linux systemd openssl \
      sed createrepo_c squashfs-tools cdrkit parted e2fsprogs dosfstools \
      xfsprogs zstd veritysetup grub2 grub2-pc systemd-ukify binutils lsof \
      python3 python3-pip jq oras && \
   tdnf clean all

# Create virtual environment and install Python dependencies for telemetry
COPY ./usr/local/bin/requirements.txt /usr/local/bin/requirements.txt
RUN python3 -m venv /opt/telemetry-venv && \
   /opt/telemetry-venv/bin/pip install --no-cache-dir -r /usr/local/bin/requirements.txt

# Copy binaries.
COPY . /

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
