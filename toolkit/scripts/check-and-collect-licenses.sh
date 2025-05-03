#!/usr/bin/env bash
# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

set -eu

REPO_ROOT=$(git rev-parse --show-toplevel)
OUTPUT_DIR="$REPO_ROOT/toolkit/out"
LICENSE_SCAN_OUTPUT="$OUTPUT_DIR/LICENSES-SCAN.json"
LICENSES_DIR="$OUTPUT_DIR/LICENSES"
TOOLS_DIR="$REPO_ROOT/toolkit/tools"

run_trivy_scan() {
  echo "Running Trivy license scan..."
  mkdir -p "$(dirname "$LICENSE_SCAN_OUTPUT")"
  trivy fs --scanners license --format json --list-all-pkgs "$REPO_ROOT" > "$LICENSE_SCAN_OUTPUT"

  echo "Checking for HIGH or CRITICAL severity licenses..."
  local output
  output=$(jq -r '.Results[] | select(.Licenses) | .Licenses[] | select(.Severity == "HIGH" or .Severity == "CRITICAL") | "- \(.PkgName) [\(.Category)]"' "$LICENSE_SCAN_OUTPUT")

  if [ -n "$output" ]; then
    echo "❌ Found HIGH or CRITICAL severity license classification:"
    echo "$output"
    exit 1
  else
    echo "✅ License check passed."
  fi
}

collect_licenses() {
  echo "Collecting license files..."
  rm -rf -- "$LICENSES_DIR"
  mkdir -p "$LICENSES_DIR"

  echo "Collecting licenses from go modules cache..."
  (cd "$TOOLS_DIR" && go mod download)

  (cd "$TOOLS_DIR" && go list -m -json all) | jq -r '.Path + " " + .Version' | \
  while IFS= read -r module_version; do
    module=$(echo "$module_version" | awk '{print $1}')
    version=$(echo "$module_version" | awk '{print $2}')
    modpath="$(go env GOMODCACHE)/${module}@${version}"

    if [ -d "$modpath" ]; then
      safe_dir=$(echo "$module" | sed 's|/|_|g')
      target_dir="$LICENSES_DIR/$safe_dir"
      mkdir -p "$target_dir"

      find "$modpath" -maxdepth 1 -type f \( -iname "LICENSE*" -o -iname "COPYING*" -o -iname "NOTICE*" \) | \
      while IFS= read -r file; do
        cp -- "$file" "$target_dir/"
      done
    fi
  done

  echo "Including toolkit license..."
  cp -- "$REPO_ROOT/LICENSE" "$LICENSES_DIR/LICENSE"

  echo "✅ License files copied to $LICENSES_DIR."
}

run_trivy_scan
collect_licenses
