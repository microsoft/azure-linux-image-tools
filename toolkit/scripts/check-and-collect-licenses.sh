#!/usr/bin/env bash
# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

set -eu

LICENSE_SCAN_JSON="out/license-scan.json"
LICENSE_SCAN_OUTPUT="$LICENSE_SCAN_JSON"
LICENSES_DIR="out/licenses"

# Scans go.mod for critical & high severity license violations which catches
# Forbidden & Restricted licenses based on
# https://trivy.dev/v0.61/docs/scanner/license
echo "Running Trivy license scan..."
mkdir -p "$(dirname "$LICENSE_SCAN_JSON")"
trivy fs --scanners license --format json --list-all-pkgs . > "$LICENSE_SCAN_JSON"

echo "Checking for HIGH or CRITICAL severity licenses..."
output=$(jq -r '.Results[] | select(.Licenses) | .Licenses[] | select(.Severity == "HIGH" or .Severity == "CRITICAL") | "- \(.PkgName) [\(.Category)]"' "$LICENSE_SCAN_OUTPUT")
if [ -n "$output" ]; then
  echo "❌ Found HIGH or CRITICAL severity license classification:"
  echo "$output"
  exit 1
else
  echo "✅ License check passed."
fi

echo "Running license-collect..."
echo "Copying license files from Go module cache..."
rm -rf "$LICENSES_DIR"
mkdir -p "$LICENSES_DIR"

cd tools
go mod download
go list -m -json all | jq -r '.Path + " " + .Version' | \
while read -r module version; do
  modpath="$(go env GOMODCACHE)/${module}@${version}"
  if [ -d "$modpath" ]; then
    find "$modpath" -maxdepth 1 -type f \( -iname "LICENSE*" -o -iname "COPYING*" -o -iname "NOTICE*" \) | \
    while read -r file; do
      safe_name=$(echo "$module" | sed 's|/|_|g')
      cp "$file" "../${LICENSES_DIR}/${safe_name}_$(basename "$file")"
    done
  fi
done

echo "Including license from the toolkit..."
REPO_ROOT=$(git rev-parse --show-toplevel)
cp $REPO_ROOT/LICENSE "../$LICENSES_DIR/LICENSE"

echo "License files copied to $LICENSES_DIR."
