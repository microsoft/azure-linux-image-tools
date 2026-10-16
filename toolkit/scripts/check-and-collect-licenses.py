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
from typing import Any, cast
import urllib
import urllib.request

SCRIPT_DIR = Path(os.path.realpath(__file__)).parent
REPO_ROOT = (SCRIPT_DIR / ".." / "..").resolve()
OUTPUT_DIR = Path(REPO_ROOT) / "toolkit" / "out"
LICENSE_SCAN_OUTPUT = OUTPUT_DIR / "LICENSES-SCAN.json"
LICENSES_DIR = OUTPUT_DIR / "LICENSES"
TOOLS_DIR = Path(REPO_ROOT) / "toolkit" / "tools"
LICENSE_CHOICES_JSON = SCRIPT_DIR / "license-choices.json"

def download_trivy():
    TRIVY_VERSION = "0.69.2"

    print("Downloading Trivy...")

    if shutil.which("trivy"):
        print("Trivy is already installed. Skipping installation.")
        return

    machine = platform.machine()
    arch = "64bit"
    expected_sha256 = "affa59a1e37d86e4b8ab2cd02f0ab2e63d22f1bf9cf6a7aa326c884e25e26ce3"
    if machine == "aarch64":
        arch = "ARM64"
        expected_sha256 = "c73b97699c317b0d25532b3f188564b4e29d13d5472ce6f8eb078082546a6481"

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

        actual_sha256 = sha256.hexdigest()

        print("Verifying checksum...")
        if actual_sha256 != expected_sha256:
            print(f"SHA256 checksum does not match! (Expected: {expected_sha256}, Actual: {actual_sha256})")
            sys.exit(1)

        with tarfile.open(tar_path, "r:gz") as tar:
            tar.extractall(path=tmpdir)

        subprocess.run(["sudo", "mv", os.path.join(tmpdir, "trivy"), BIN_PATH], check=True)
        os.remove(tar_path)

    print("Trivy installed successfully.")

def validate_license_choice(package_id: str, choice: Any) -> None:
    """Raise ValueError for malformed or internally inconsistent policy entries."""
    package_name, separator, version = package_id.rpartition("@")
    if not package_name.strip() or not separator or not version.strip():
        raise ValueError(f"{package_id!r}: license choice key must use 'package@version'")

    if not isinstance(choice, dict):
        raise ValueError(f"{package_id}: license choice must be an object")
    choice = cast(dict[str, Any], choice)

    for field in ("selected", "source"):
        value = choice.get(field)
        if not isinstance(value, str) or not value.strip():
            raise ValueError(f"{package_id}: '{field}' must be a non-empty string")

    reviewed_licenses = choice.get("licenses")
    if not isinstance(reviewed_licenses, list):
        raise ValueError(f"{package_id}: 'licenses' must be a non-empty list of license names")
    reviewed_licenses = cast(list[Any], reviewed_licenses)
    if not reviewed_licenses or any(
        not isinstance(license_name, str) or not license_name.strip()
        for license_name in reviewed_licenses
    ):
        raise ValueError(f"{package_id}: 'licenses' must be a non-empty list of license names")

    selected_license = choice["selected"]
    if selected_license not in reviewed_licenses:
        raise ValueError(
            f"{package_id}: selected license {selected_license!r} is not in the reviewed licenses"
        )

def find_license_choice(
    package: dict[str, Any],
    license_name: str | None,
    license_choices: dict[str, dict[str, Any]],
) -> tuple[str, str, str] | None:
    """Return the package ID, selected license, and source URL if the policy allows ignoring the flagged license."""
    package_id = f"{package.get('Name')}@{package.get('Version')}"
    if package_id not in license_choices:
        return None

    expected_licenses = set(license_choices[package_id]["licenses"])
    if set(package.get("Licenses", [])) != expected_licenses:
        return None

    selected_license = license_choices[package_id]["selected"]
    unselected_licenses = expected_licenses - {selected_license}
    if license_name in unselected_licenses:
        license_source_url = license_choices[package_id]["source"]
        return package_id, selected_license, license_source_url

    return None

def run_trivy_scan():
    print("Running Trivy license scan...")

    with open(LICENSE_CHOICES_JSON) as policy_file:
        license_choices = json.load(policy_file)
    if not isinstance(license_choices, dict):
        raise ValueError("License choices must be an object keyed by 'package@version'")
    license_choices = cast(dict[str, dict[str, Any]], license_choices)
    for package_id, choice in license_choices.items():
        validate_license_choice(package_id, choice)

    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    with open(LICENSE_SCAN_OUTPUT, "w") as out_file:
        subprocess.run(
            ["trivy", "fs", "--scanners", "license", "--format", "json", "--list-all-pkgs", REPO_ROOT],
            check=True,
            stdout=out_file,
        )

    blocked_license_severities = {"HIGH", "CRITICAL"}
    acceptable_license_severities = {"LOW", "MEDIUM", "HIGH", "CRITICAL"} - blocked_license_severities
    blocked_severities_text = " or ".join(sorted(blocked_license_severities))

    print(f"Checking for {blocked_severities_text} severity licenses...")
    with open(LICENSE_SCAN_OUTPUT) as f:
        data = json.load(f)

    findings: list[str] = []
    results = data.get("Results", [])

    packages_by_target_and_package_name: dict[tuple[str, str], list[dict[str, Any]]] = {}
    severity_by_target_package_and_license_name: dict[tuple[str, str, str], str | None] = {}
    for result in results:
        target = result.get("Target")
        if not target:
            continue

        for package in result.get("Packages", []):
            package_name = package.get("Name")
            if package_name:
                key = (target, package_name)
                packages_by_target_and_package_name.setdefault(key, []).append(package)

        for license_entry in result.get("Licenses", []):
            license_key = (target, license_entry.get("PkgName"), license_entry.get("Name"))
            severity = license_entry.get("Severity")
            if license_key in severity_by_target_package_and_license_name:
                previous_severity = severity_by_target_package_and_license_name[license_key]
                if previous_severity != severity:
                    raise ValueError(f"Conflicting license severities for {license_key}: {previous_severity!r} and {severity!r}")

            severity_by_target_package_and_license_name[license_key] = severity

    for result in results:
        for license_entry in result.get("Licenses", []):
            if license_entry.get("Severity") in blocked_license_severities:
                package_name = license_entry.get('PkgName')
                license_name = license_entry.get('Name')

                packages = packages_by_target_and_package_name.get((result.get("Target"), package_name), [])
                if len(packages) == 0:
                    raise ValueError(f"No packages found for target {result.get('Target')} and package name {package_name}")
                if len(packages) > 1:
                    raise ValueError(f"Ambiguous package match for target {result.get('Target')} and package name {package_name}: found {len(packages)} packages")

                license_choice_match = find_license_choice(packages[0], license_name, license_choices)
                if license_choice_match:
                    package_id, selected_license, license_source_url = license_choice_match

                    selected_license_key = (result.get("Target"), package_name, selected_license)
                    if selected_license_key not in severity_by_target_package_and_license_name:
                        raise ValueError(f"Missing severity for selected license {selected_license_key}")

                    selected_license_severity = severity_by_target_package_and_license_name[selected_license_key]
                    if selected_license_severity in acceptable_license_severities:
                        print(f"Using {selected_license} for {package_id} ({license_source_url})")
                        continue

                category = license_entry.get('Category')
                findings.append(f"- {package_name}: {license_name} [{category}]")

    if findings:
        print(f"❌ Found {blocked_severities_text} severity license classification:")
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
