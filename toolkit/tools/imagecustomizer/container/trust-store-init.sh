#!/usr/bin/env bash

# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.
#
# Shared trust-store initialisation for the imagecustomizer container.
#
# Both entrypoint.sh (default flow) and run.sh (pull-and-customize flow)
# source this file and call `imagecustomizer_init_trust_store`. The function
# is idempotent.
#
# It performs, in order:
#   1. Install any customer-supplied trust anchors that have been
#      bind-mounted into the container (see env vars below).
#   2. Refresh the system ca-certificates package from PMC so that
#      certificates that have been rotated upstream since the container
#      was built are picked up. This handles the case where the
#      container tag is pinned to an older release.
#   3. Re-extract the unified trust store so every TLS client in the
#      container (oras, tdnf, openssl, curl, Go's crypto/tls, python
#      via SSL_CERT_FILE) sees the new roots.
#
# Environment variables:
#   IMAGECUSTOMIZER_TRUST_REFRESH
#       auto    Default. Try to refresh ca-certificates; on failure
#               log a warning and continue with the bundled trust store.
#       off     Do nothing. For fully offline / air-gapped runs.
#       strict  Try to refresh; fail the container start if refresh fails.
#
#   IMAGECUSTOMIZER_HOST_CA_DIR
#       Directory inside the container that holds host-provided trust
#       anchors in PEM form. Defaults to /host-ssl/certs. Files there
#       are copied into the system anchor directory before refresh.
#
#   IMAGECUSTOMIZER_PMC_BASEURL
#       If set, rewrite the Azure Linux base repo's baseurl before
#       attempting to refresh ca-certificates. Lets a customer point at
#       a private PMC mirror.

set -u

readonly IMAGECUSTOMIZER_TRUST_SOURCE_ANCHORS=/etc/pki/ca-trust/source/anchors
readonly IMAGECUSTOMIZER_USER_ANCHOR_SUBDIR="${IMAGECUSTOMIZER_TRUST_SOURCE_ANCHORS}/imagecustomizer-user"
readonly IMAGECUSTOMIZER_PMC_REPO_FILE=/etc/yum.repos.d/azurelinux-official-base.repo

_imagecustomizer_log_warn() {
    echo "imagecustomizer-trust-store: warn: $*" >&2
}

_imagecustomizer_log_info() {
    echo "imagecustomizer-trust-store: $*" >&2
}

_imagecustomizer_install_user_anchors() {
    local host_dir="${IMAGECUSTOMIZER_HOST_CA_DIR:-/host-ssl/certs}"

    if [[ ! -d "$host_dir" ]]; then
        return 0
    fi

    mkdir -p "$IMAGECUSTOMIZER_USER_ANCHOR_SUBDIR"

    # Accept .pem and .crt; copy with a stable prefix so we can identify
    # them later. Failing to copy individual files must not abort the run.
    local copied=0
    local f
    for f in "$host_dir"/*.pem "$host_dir"/*.crt; do
        [[ -e "$f" ]] || continue
        if cp -f "$f" "$IMAGECUSTOMIZER_USER_ANCHOR_SUBDIR/" 2>/dev/null; then
            copied=$((copied + 1))
        fi
    done

    if (( copied > 0 )); then
        _imagecustomizer_log_info "installed $copied user-supplied trust anchor(s) from $host_dir"
    fi
}

_imagecustomizer_apply_pmc_override() {
    local override="${IMAGECUSTOMIZER_PMC_BASEURL:-}"
    if [[ -z "$override" ]]; then
        return 0
    fi

    if [[ ! -f "$IMAGECUSTOMIZER_PMC_REPO_FILE" ]]; then
        _imagecustomizer_log_warn "IMAGECUSTOMIZER_PMC_BASEURL set but $IMAGECUSTOMIZER_PMC_REPO_FILE not found; skipping"
        return 0
    fi

    # Rewrite the baseurl line in place. We deliberately leave the
    # gpgkey / gpgcheck settings alone so the customer mirror still has
    # to be signed by the Azure Linux GPG key.
    sed -i -E "s|^baseurl=.*|baseurl=${override}|g" "$IMAGECUSTOMIZER_PMC_REPO_FILE"
    _imagecustomizer_log_info "rewrote PMC baseurl to $override"
}

_imagecustomizer_refresh_ca_certificates() {
    # `tdnf --refresh install` forces a metadata refresh even though
    # ca-certificates is already installed; this is what actually pulls
    # the new roots.
    tdnf install -y --refresh ca-certificates >/dev/null 2>&1
}

_imagecustomizer_extract_trust() {
    # Regenerate /etc/pki/ca-trust/extracted/* which is what openssl,
    # curl, Go's crypto/tls, and (via SSL_CERT_FILE) python all read.
    if command -v update-ca-trust >/dev/null 2>&1; then
        update-ca-trust extract >/dev/null 2>&1 || return 1
    fi
    return 0
}

# Public entry point. Safe to call multiple times.
imagecustomizer_init_trust_store() {
    local mode="${IMAGECUSTOMIZER_TRUST_REFRESH:-auto}"

    # Layer 2: user-supplied anchors. Always attempted (cheap, opt-in
    # via bind mount). Done before refresh so that, if the customer
    # has bind-mounted the new MCR/PMC root, the refresh itself can use it.
    _imagecustomizer_install_user_anchors
    _imagecustomizer_apply_pmc_override
    _imagecustomizer_extract_trust || true

    # Layer 1: PMC-driven refresh.
    case "$mode" in
        off)
            _imagecustomizer_log_info "IMAGECUSTOMIZER_TRUST_REFRESH=off; skipping ca-certificates refresh"
            ;;
        strict)
            if ! _imagecustomizer_refresh_ca_certificates; then
                _imagecustomizer_log_warn "ca-certificates refresh failed under IMAGECUSTOMIZER_TRUST_REFRESH=strict"
                return 1
            fi
            _imagecustomizer_extract_trust || {
                _imagecustomizer_log_warn "update-ca-trust extract failed under IMAGECUSTOMIZER_TRUST_REFRESH=strict"
                return 1
            }
            _imagecustomizer_log_info "ca-certificates refreshed (strict mode)"
            ;;
        auto|"")
            if _imagecustomizer_refresh_ca_certificates; then
                _imagecustomizer_extract_trust || true
                _imagecustomizer_log_info "ca-certificates refreshed"
            else
                _imagecustomizer_log_warn "ca-certificates refresh failed; using bundled trust store"
            fi
            ;;
        *)
            _imagecustomizer_log_warn "unrecognised IMAGECUSTOMIZER_TRUST_REFRESH='$mode'; treating as 'auto'"
            if _imagecustomizer_refresh_ca_certificates; then
                _imagecustomizer_extract_trust || true
            fi
            ;;
    esac

    # Make sure the python telemetry venv (which uses certifi by default)
    # picks up the system store as well. Exporting these is harmless for
    # everything else.
    export SSL_CERT_FILE=/etc/pki/tls/certs/ca-bundle.crt
    export REQUESTS_CA_BUNDLE=/etc/pki/tls/certs/ca-bundle.crt

    return 0
}
