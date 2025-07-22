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
RUN python3 -m venv /opt/telemetry-venv
COPY telemetry-requirements.txt /telemetry-requirements.txt
RUN /opt/telemetry-venv/bin/pip install --no-cache-dir -r /telemetry-requirements.txt
RUN rm -rf /telemetry-requirements.txt

# Copy all necessary files
COPY .mariner-toolkit-ignore-dockerenv /
COPY usr /usr

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
