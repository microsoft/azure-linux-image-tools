#!/usr/bin/env bash

# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

set -e

ENABLE_TELEMETRY="${ENABLE_TELEMETRY:-true}"
HELP=false

# Check if --disable-telemetry flag is present in arguments
for arg in "$@"; do
    case "$arg" in
        "--disable-telemetry") ENABLE_TELEMETRY=false;;
        "--help") HELP=true;;
        "--version") HELP=true;;
    esac
done

# Start telemetry service if enabled and connection string is set
if [[ "$ENABLE_TELEMETRY" == "true" ]] && [[ -n "$AZURE_MONITOR_CONNECTION_STRING" ]]; then

    export OTEL_PORT=4317
    export OTEL_EXPORTER_OTLP_PROTOCOL="grpc"
    export OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:${OTEL_PORT}"

    /usr/lib/imagecustomizer/telemetry-venv/bin/python /usr/lib/imagecustomizer/telemetry_hopper.py --port $OTEL_PORT > /var/log/image_customizer_telemetry.log 2>&1 || true &
    sleep 1
fi

if [[ "$HELP" == "false" ]]; then
    # containerd by default creates /dev as a tmpfs and populates it with a copy of the host's /dev at the time of
    # container creation. Replace it with a real devtmpfs so that partitions are populated when a virtual disk is
    # mounted.
    mount -t devtmpfs devtmpfs /dev
fi

imagecustomizer "$@"
