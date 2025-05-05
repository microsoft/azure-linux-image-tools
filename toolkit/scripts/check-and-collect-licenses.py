#!/usr/bin/env python3
# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import hashlib
import io
import json
from pathlib import Path
import shutil
import subprocess
import sys

import tarfile
import urllib

REPO_ROOT = subprocess.run(["git", "rev-parse", "--show-toplevel"], capture_output=True, text=True, check=True).stdout.strip()
OUTPUT_DIR = Path(REPO_ROOT) / "toolkit" / "out"
LICENSE_SCAN_OUTPUT = OUTPUT_DIR / "LICENSES-SCAN.json"
LICENSES_DIR = OUTPUT_DIR / "LICENSES"
TOOLS_DIR = Path(REPO_ROOT) / "toolkit" / "tools"

def download_trivy():
    TRIVY_VERSION = "0.61.1"
    TRIVY_FILENAME = f"trivy_{TRIVY_VERSION}_Linux-64bit.tar.gz"
    TRIVY_URL = f"https://github.com/aquasecurity/trivy/releases/download/v{TRIVY_VERSION}/{TRIVY_FILENAME}"
    EXPECTED_SHA256 = "dcc6f48c383833f3a8ee0380f7990a17c89e36553cdf34e1f2d3159f9d8270ec"

    print("Downloading Trivy...")

    if shutil.which("trivy"):
        print("Trivy is already installed. Skipping installation.")
        return

    urllib.request.urlretrieve(TRIVY_URL, TRIVY_FILENAME)

    print("Verifying checksum...")
    sha256 = hashlib.sha256()
    with open(TRIVY_FILENAME, "rb") as f:
        for chunk in iter(lambda: f.read(4096), b""):
            sha256.update(chunk)
    if sha256.hexdigest() != EXPECTED_SHA256:
        print("SHA256 checksum does not match!")
        sys.exit(1)

    # Extract
    print("Extracting archive...")
    with tarfile.open(TRIVY_FILENAME, "r:gz") as tar:
        tar.extractall()

    print("Moving Trivy binary to /usr/local/bin...")
    shutil.move("trivy", "/usr/local/bin/trivy")

    print("Trivy installed successfully.")

def run_trivy_scan():
    print("Running Trivy license scan...")
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    with open(LICENSE_SCAN_OUTPUT, "w") as out_file:
        subprocess.run(
            ["trivy", "fs", "--scanners", "license", "--format", "json", "--list-all-pkgs", REPO_ROOT],
            check=True,
            stdout=out_file,
        )

    print("Checking for HIGH or CRITICAL severity licenses...")
    with open(LICENSE_SCAN_OUTPUT) as f:
        data = json.load(f)

    findings = []
    for result in data.get("Results", []):
        for license_entry in result.get("Licenses", []):
            if license_entry.get("Severity") in ("HIGH", "CRITICAL"):
                findings.append(f"- {license_entry.get('PkgName')} [{license_entry.get('Category')}]")

    if findings:
        print("❌ Found HIGH or CRITICAL severity license classification:")
        print("\n".join(findings))
        sys.exit(1)
    else:
        print("✅ License check passed.")

def parse_go_modules_json_stream(output: str):
    modules = []
    buffer = ""
    brace_count = 0

    for line in io.StringIO(output):
        line = line.rstrip()
        if not line:
            continue
        brace_count += line.count('{') - line.count('}')
        buffer += line + "\n"
        if brace_count == 0 and buffer.strip():
            try:
                mod = json.loads(buffer)
                if "Path" in mod and "Version" in mod:
                    modules.append((mod["Path"], mod["Version"]))
            except json.JSONDecodeError as e:
                print(f"Warning: Skipping JSON block due to decode error: {e}")
            buffer = ""

    return modules

def collect_licenses():
    print("Collecting license files...")
    if LICENSES_DIR.exists():
        shutil.rmtree(LICENSES_DIR)
    LICENSES_DIR.mkdir(parents=True)

    print("Collecting licenses from Go modules cache...")
    subprocess.run(["go", "mod", "download"], cwd=TOOLS_DIR, check=True)

    proc = subprocess.run(
        ["go", "list", "-m", "-json", "all"],
        cwd=TOOLS_DIR,
        check=True,
        stdout=subprocess.PIPE,
        text=True,
    )
    modules = parse_go_modules_json_stream(proc.stdout)

    gomodcache = subprocess.run(["go", "env", "GOMODCACHE"], check=True, stdout=subprocess.PIPE, text=True).stdout.strip()

    for module, version in modules:
        modpath = Path(gomodcache) / f"{module}@{version}"
        if not modpath.exists():
            continue

        target_dir = LICENSES_DIR / module
        target_dir.mkdir(parents=True, exist_ok=True)

        for license_name in ["LICENSE", "COPYING", "NOTICE"]:
            for file in modpath.glob(f"{license_name}*"):
                if file.is_file():
                    shutil.copy(file, target_dir / file.name)

    print("Including toolkit license...")
    shutil.copy(Path(REPO_ROOT) / "LICENSE", LICENSES_DIR / "LICENSE")

    print(f"✅ License files copied to {LICENSES_DIR}.")

if __name__ == "__main__":
    download_trivy()
    run_trivy_scan()
    collect_licenses()
