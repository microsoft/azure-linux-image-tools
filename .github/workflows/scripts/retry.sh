#!/bin/bash
# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

set -u

# The script runs a command in a retry loop.
#
# Usage:
#
#   retry.sh RETRY_COUNT SLEEP_TIME COMMAND ARGS...

if [ $# -lt 3 ]; then
    echo "Usage: retry.sh RETRY_COUNT SLEEP_TIME COMMAND [ARGS...]" >&2
    exit 1
fi

RETRY_COUNT="$1"
SLEEP_TIME="$2"

shift 2

for ((i = 1; ; i++)); do
    "$@" \
        && break \
        || err=$?

    if [ "$i" == "$RETRY_COUNT" ]; then
        echo "Error: command failed after $RETRY_COUNT attempt(s): $*" >&2
        exit $err
    fi

    echo "Retry $i/$RETRY_COUNT: command failed (exit code $err), retrying in ${SLEEP_TIME}s..." >&2
    sleep "$SLEEP_TIME"
done
