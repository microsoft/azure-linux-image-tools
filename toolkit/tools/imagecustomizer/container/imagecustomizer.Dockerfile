# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

ARG BASE_IMAGE="mcr.microsoft.com/azurelinux/base/core:3.0"

FROM ${BASE_IMAGE}
RUN tdnf update -y && \
   tdnf install -y qemu-img rpm coreutils util-linux systemd openssl \
      sed createrepo_c squashfs-tools cdrkit parted e2fsprogs dosfstools \
      xfsprogs zstd veritysetup grub2 grub2-pc systemd-ukify binutils lsof \
   python3 python3-pip && \
   tdnf clean all

COPY . /

ENV OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4317" \
   OTEL_EXPORTER_OTLP_PROTOCOL="grpc"

# Create virtual environment and install Python dependencies for telemetry
RUN python3 -m venv /opt/telemetry-venv && \
   /opt/telemetry-venv/bin/pip install --no-cache-dir -r /usr/local/bin/requirements.txt

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
