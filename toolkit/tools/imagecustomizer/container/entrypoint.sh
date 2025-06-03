#!/usr/bin/env bash
set -e

ENABLE_TELEMETRY="${ENABLE_TELEMETRY:-true}"

# Check if --disable-telemetry flag is present in arguments
for arg in "$@"; do
    if [[ "$arg" == "--disable-telemetry" ]]; then
        ENABLE_TELEMETRY=false
        break
    fi
done

# Start telemetry service if enabled
if [[ "$ENABLE_TELEMETRY" == "true" ]]; then
    /opt/telemetry-venv/bin/python /usr/local/bin/telemetry_hopper.py > /dev/null 2>&1 || true &
    sleep 1
fi

exec "$@"
