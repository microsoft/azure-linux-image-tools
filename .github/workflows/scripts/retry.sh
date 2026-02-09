# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

# The script runs a command in a retry loop.
#
# Usage:
#
#   retry.sh RETRY_COUNT SLEEP_TIME COMMAND ARGS...

RETRY_COUNT="$1"
SLEEP_TIME="$2"

shift 2

for ((i = 1; ; i++)); do
    "$@" \
        && break \
        || err=$?

    if [ "$i" == "$RETRY_COUNT" ]; then
        exit $err
    fi

    sleep "$SLEEP_TIME"
done
