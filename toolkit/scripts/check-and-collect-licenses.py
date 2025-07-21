#!/usr/bin/env python3
# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import hashlib
import io
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess
import sys
import tarfile
import tempfile
import urllib
import urllib.request

SCRIPT_DIR = Path(os.path.realpath(__file__)).parent
REPO_ROOT = (SCRIPT_DIR / ".." / "..").resolve()
OUTPUT_DIR = Path(REPO_ROOT) / "toolkit" / "out"
LICENSE_SCAN_OUTPUT = OUTPUT_DIR / "LICENSES-SCAN.json"
LICENSES_DIR = OUTPUT_DIR / "LICENSES"
TOOLS_DIR = Path(REPO_ROOT) / "toolkit" / "tools"

def download_trivy():
    TRIVY_VERSION = "0.61.1"
    TRIVY_FILENAME = f"trivy_{TRIVY_VERSION}_Linux-64bit.tar.gz"
    TRIVY_URL = f"https://github.com/aquasecurity/trivy/releases/download/v{TRIVY_VERSION}/{TRIVY_FILENAME}"

    print("Downloading Trivy...")

    if shutil.which("trivy"):
        print("Trivy is already installed. Skipping installation.")
        return

    machine = platform.machine()
    arch = "64bit"
    expected_sha256 = "dcc6f48c383833f3a8ee0380f7990a17c89e36553cdf34e1f2d3159f9d8270ec"
    if machine == "aarch64":
        arch = "ARM64"
        expected_sha256 = "a2f28808626ae08877279e13a32d9cc8f845bdfdc762a692b2f32768326208a6"

    TRIVY_FILENAME = f"trivy_{TRIVY_VERSION}_Linux-{arch}.tar.gz"
    TRIVY_URL = f"https://github.com/aquasecurity/trivy/releases/download/v{TRIVY_VERSION}/{TRIVY_FILENAME}"
    BIN_PATH = "/usr/local/bin/trivy"

    with tempfile.TemporaryDirectory() as tmpdir:
        tar_path = os.path.join(tmpdir, TRIVY_FILENAME)

        try:
            urllib.request.urlretrieve(TRIVY_URL, tar_path)
        except Exception as e:
            print(f"Download Trivy failed: {e}")
            sys.exit(1)

        sha256 = hashlib.sha256()
        with open(tar_path, "rb") as f:
            for chunk in iter(lambda: f.read(4096), b""):
                sha256.update(chunk)

        print("Verifying checksum...")
        if sha256.hexdigest() != expected_sha256:
            print("SHA256 checksum does not match!")
            sys.exit(1)

        with tarfile.open(tar_path, "r:gz") as tar:
            tar.extractall(path=tmpdir)

        subprocess.run(["sudo", "mv", os.path.join(tmpdir, "trivy"), BIN_PATH], check=True)
        os.remove(tar_path)

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
