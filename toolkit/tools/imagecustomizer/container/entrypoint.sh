#!/usr/bin/env bash

# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

set -e

ENABLE_TELEMETRY="${ENABLE_TELEMETRY:-true}"
HELP=false
TELEMETRY_PID=""

function stopTelemetry() {
    if [[ -n "$TELEMETRY_PID" ]]; then
        if kill -0 "$TELEMETRY_PID" 2>/dev/null; then
            if ! kill -TERM "$TELEMETRY_PID" 2>/dev/null; then
                echo "Failed to signal telemetry collector shutdown" >&2
            fi
        fi
        if ! wait "$TELEMETRY_PID"; then
            echo "Telemetry collector failed; see /var/log/image_customizer_telemetry.log" >&2
        fi
        TELEMETRY_PID=""
    fi
}
trap stopTelemetry EXIT

# Check if --disable-telemetry flag is present in arguments
for arg in "$@"; do
    case "$arg" in
        "--disable-telemetry") ENABLE_TELEMETRY=false;;
        "--help") HELP=true;;
        "--version") HELP=true;;
    esac
done

# Start telemetry service if enabled and connection string is set
if [[ "$HELP" == "false" ]] && [[ "$ENABLE_TELEMETRY" == "true" ]] && [[ -n "$AZURE_MONITOR_CONNECTION_STRING" ]]; then
    export OTEL_EXPORTER_OTLP_PROTOCOL="grpc"
    export OTEL_EXPORTER_OTLP_ENDPOINT="http://127.0.0.1:4317"

    /usr/lib/imagecustomizer/telemetrycollector > /var/log/image_customizer_telemetry.log 2>&1 &
    TELEMETRY_PID=$!
    TELEMETRY_READY=false
    for ((attempt = 0; attempt < 50; attempt++)); do
        if ! kill -0 "$TELEMETRY_PID" 2>/dev/null; then
            break
        fi
        if (: > /dev/tcp/127.0.0.1/4317) 2>/dev/null; then
            TELEMETRY_READY=true
            break
        fi
        sleep 0.1
    done
    if [[ "$TELEMETRY_READY" == "false" ]]; then
        echo "Telemetry collector did not start; see /var/log/image_customizer_telemetry.log" >&2
        stopTelemetry
        unset OTEL_EXPORTER_OTLP_ENDPOINT
    fi
fi

if [[ "$HELP" == "false" ]]; then
    # containerd by default creates /dev as a tmpfs and populates it with a copy of the host's /dev at the time of
    # container creation. Replace it with a real devtmpfs so that partitions are populated when a virtual disk is
    # mounted.
    mount -t devtmpfs devtmpfs /dev
fi

imagecustomizer "$@"
