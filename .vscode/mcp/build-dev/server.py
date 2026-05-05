#!/usr/bin/env python3
"""MCP server that triggers and monitors the 'Build (dev)' GitHub Actions workflow."""

import json
import os
import re
import shutil
import subprocess
import tempfile
import time
import xml.etree.ElementTree as ET
import zipfile
from typing import Iterator, cast

from mcp.server.fastmcp import FastMCP


mcp = FastMCP("azure-linux-image-tools")

GH_REPO = "microsoft/azure-linux-image-tools"
DEFAULT_BRANCH = "main"
ANSI_RE = re.compile(r"\x1b\[[0-9;]*m")
ARTIFACTS_PER_PAGE = 100
ARTIFACTS_MAX_PAGES = 100
FAILED_REPORT_SUFFIX = ".failed.txt"
SKIPPED_REPORT_SUFFIX = ".skipped.txt"

# Safety caps for polling loops.
WATCH_MAX_CONSECUTIVE_ERRORS = 10
WATCH_DEFAULT_TIMEOUT_SECONDS = 6 * 60 * 60  # 6 hours

# Repo root resolved from this file's location: <repo>/.vscode/mcp/build-dev/server.py.
REPO_DIR = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", ".."))
DOWNLOAD_IMAGE_SH = f"{REPO_DIR}/.github/workflows/scripts/download-image.sh"
BASE_IMAGE_DIR = os.path.join(REPO_DIR, "test", "base-images")
BASE_IMAGE_STORAGE_ACCOUNT = "maritimusgithubstorage"
BASE_IMAGE_CONTAINER = "os-images-cache"
BASE_IMAGE_SUBSCRIPTION = "b3e01d89-bd55-414f-bbb4-cdfeb2628caa"

def _strip_ansi(text: str) -> str:
    return ANSI_RE.sub("", text)


def _run(cmd: list[str], text: bool = True, cwd: str | None = None) -> subprocess.CompletedProcess[str|bytes]:
    return subprocess.run(cmd, capture_output=True, text=text, check=True, cwd=cwd)


def _watch_run(
    run_id: str,
    include_jobs: bool = False,
    poll_interval: int = 30,
    timeout_seconds: int = WATCH_DEFAULT_TIMEOUT_SECONDS,
) -> Iterator[dict[str, object]]:
    """Yield `gh run view` JSON snapshots every `poll_interval` seconds.

    Yields at least once per poll. After yielding a snapshot whose status is
    'completed' the generator stops. Raises `TimeoutError` if the run does not
    complete within `timeout_seconds`, or `RuntimeError` after
    `WATCH_MAX_CONSECUTIVE_ERRORS` consecutive `gh`/JSON errors.
    """
    fields = "status,conclusion,jobs" if include_jobs else "status,conclusion"
    deadline = time.monotonic() + timeout_seconds
    consecutive_errors = 0
    last_error: Exception | None = None

    while True:
        if time.monotonic() > deadline:
            raise TimeoutError(
                f"Run {run_id} did not complete within {timeout_seconds} seconds."
            )

        try:
            result = _run([
                "gh", "run", "view", run_id, "--repo", GH_REPO,
                "--json", fields,
            ])
            run_json = json.loads(result.stdout)
        except (subprocess.CalledProcessError, json.JSONDecodeError) as e:
            last_error = e
            consecutive_errors += 1
            if consecutive_errors >= WATCH_MAX_CONSECUTIVE_ERRORS:
                raise RuntimeError(
                    f"Giving up on run {run_id} after {consecutive_errors} "
                    f"consecutive errors polling `gh run view`: {last_error}"
                ) from last_error
            time.sleep(poll_interval)
            continue

        consecutive_errors = 0
        yield run_json

        if run_json.get("status") == "completed":
            return

        time.sleep(poll_interval)


def _get_upstream(repo_dir: str) -> str:
    """Return the upstream branch name for the repository's currently checked-out branch.

    Resolves `branch.<local>.merge` from git config and strips the `refs/heads/`
    prefix, yielding a name suitable for `gh --ref` (e.g. `user/foo/my-branch`).
    Falls back to `DEFAULT_BRANCH` when HEAD is detached. Raises if the merge
    ref is missing or has an unexpected form.
    """
    local = _run(["git", "rev-parse", "--abbrev-ref", "HEAD"], cwd=repo_dir).stdout.strip()
    if local == "HEAD":
        return DEFAULT_BRANCH # Detached HEAD, fallback to default.

    merge_proc = _run(["git", "config", "--get", f"branch.{local}.merge"], cwd=repo_dir)
    merge = merge_proc.stdout.strip()    
    merge = cast(str, merge) # Can't be inferred, true because text=True in _run.

    if not merge.startswith("refs/heads/"):
        raise Exception(
            f"Unexpected merge ref of branch {local!r} in repository {repo_dir!r}: {merge!r} "
            f"(expected to start with refs/heads/)"
        )

    branch = merge.removeprefix("refs/heads/")
    return branch


def _download_artifacts(run_id: str, dest_dir: str | None = None) -> str:
    """Download all non-expired artifacts for a run into a directory.

    If dest_dir is None, downloads into a new temp directory.
    Returns the path to the destination directory.
    """
    if dest_dir is None:
        tmp_dir = tempfile.mkdtemp(prefix=f"gha-logs-{run_id}-")
    else:
        tmp_dir = dest_dir
        os.makedirs(tmp_dir, exist_ok=True)

    for page in range(1, ARTIFACTS_MAX_PAGES + 1):
        result = _run(
            ["gh", "api", f"repos/{GH_REPO}/actions/runs/{run_id}/artifacts?per_page={ARTIFACTS_PER_PAGE}&page={page}"],
        )
        data = json.loads(result.stdout)
        artifacts = data.get("artifacts", [])

        for artifact in artifacts:
            if artifact.get("expired"):
                continue

            name = artifact["name"]
            aid = artifact["id"]
            zip_dest = os.path.join(tmp_dir, name)

            zip_proc = _run(["gh", "api", f"repos/{GH_REPO}/actions/artifacts/{aid}/zip"], text=False)
            zip_bytes = cast(bytes, zip_proc.stdout) # Can't be inferred, true because text=False in _run.

            zip_temp = os.path.join(tmp_dir, f"{name}.zip")
            try:
                with open(zip_temp, "wb") as f:
                    f.write(zip_bytes)

                os.makedirs(zip_dest, exist_ok=True)
                with zipfile.ZipFile(zip_temp, "r") as zf:
                    zf.extractall(zip_dest)
            finally:
                if os.path.exists(zip_temp):
                    os.unlink(zip_temp)

        if len(artifacts) < ARTIFACTS_PER_PAGE:
            break
    else:
        raise RuntimeError(
            f"Exceeded ARTIFACTS_MAX_PAGES={ARTIFACTS_MAX_PAGES} while downloading "
            f"artifacts for run {run_id}; results may be truncated."
        )

    return tmp_dir


def _parse_go_test_results(text: str, marker: str) -> list[dict[str, str]]:
    """Parse Go test output and extract blocks terminated by '--- {marker}:'.

    A block starts at a top-level '=== RUN' (column 0) and includes all
    nested subtest '=== RUN' lines, body output, and any indented child
    '--- <result>:' lines. The block ends at the matching top-level
    '--- <result>:' line (column 0). Blocks are emitted only when the
    top-level result line matches `marker`, but the captured output
    preserves all sibling subtest content (failures, skips, and logs)
    so the caller sees the full picture rather than just the last
    subtest's tail.

    Top-level vs subtest is distinguished by indentation: Go's testing
    package emits top-level result lines at column 0 and subtest result
    lines indented by spaces. Both top-level and subtest '=== RUN' lines
    start at column 0, so subtest '=== RUN' lines are recognised by
    being seen while a block is already open, and are simply appended.
    """
    text = _strip_ansi(text)
    results: list[dict[str, str]] = []
    buf: list[str] = []
    # State machine: "idle" (no open block), "in_block" (collecting body
    # before the parent result line), "in_results" (parent result line
    # seen; trailing nested '--- ' lines may still be appended until a
    # new top-level '=== RUN' starts a sibling block).
    state = "idle"
    matched_test = ""
    run_re = re.compile(r"^=== RUN\s+")
    # Top-level result lines have no leading whitespace; subtest result
    # lines are indented by Go's testing package.
    top_result_re = re.compile(r"^--- \w+:")
    marker_re = re.compile(rf"^--- {marker}:\s+(\S+)")

    for line in text.splitlines():
        if run_re.match(line):
            if state == "in_results":
                # Sibling top-level run starting; close the previous block.
                if matched_test:
                    results.append({
                        "test": matched_test,
                        "output": "\n".join(buf),
                    })
                    matched_test = ""

                buf = [line]
                state = "in_block"
            elif state == "in_block":
                # Nested subtest '=== RUN' inside an open block; preserve it.
                buf.append(line)
            else:  # state == "idle"
                buf = [line]
                state = "in_block"
            continue

        if state == "in_block" and top_result_re.match(line):
            buf.append(line)
            m = marker_re.match(line)
            if m:
                matched_test = m.group(1)
            state = "in_results"
            continue

        if state in ("in_block", "in_results"):
            buf.append(line)

    if state in ("in_block", "in_results"):
        if matched_test:
            results.append({
                "test": matched_test,
                "output": "\n".join(buf),
            })
            matched_test = ""

    return results


def _parse_junit_xml_failures(xml_path: str) -> list[dict[str, str]]:
    """Parse a JUnit XML report and extract failure/error test cases."""
    failures: list[dict[str, str]] = []
    try:
        tree = ET.parse(xml_path)
    except (ET.ParseError, FileNotFoundError):
        return failures

    for tc in tree.iter("testcase"):
        name = tc.get("name", "unknown")
        classname = tc.get("classname", "")

        fail_el = tc.find("failure")
        err_el = tc.find("error")
        element = fail_el if fail_el is not None else err_el
        if element is None:
            continue

        message = element.get("message", "")
        body = (element.text or "").strip()
        output_parts: list[str] = []
        if message:
            output_parts.append(message)
        if body:
            output_parts.append(body)

        full_name = f"{classname}::{name}" if classname else name
        failures.append({
            "test": full_name,
            "output": "\n".join(output_parts),
        })

    return failures


def _parse_pytest_log_failures(text: str) -> list[dict[str, str]]:
    """Parse pytest verbose log output for FAILED lines and short tracebacks."""
    failures: list[dict[str, str]] = []
    lines = text.splitlines()

    # Look for the short test summary / FAILURES section.
    in_failures_section = False
    current_block: list[str] = []
    current_test = ""

    for line in lines:
        stripped = line.strip()

        # Detect start of FAILURES section.
        if re.match(r"^=+ FAILURES =+$", stripped):
            in_failures_section = True
            continue

        # Detect end of FAILURES section (short test summary or another = section).
        if in_failures_section and re.match(r"^=+ ", stripped) and "FAILURES" not in stripped:
            if current_test:
                failures.append({
                    "test": current_test,
                    "output": "\n".join(current_block),
                })
            in_failures_section = False
            continue

        if in_failures_section:
            # New test failure header: ___ test_name ___
            m = re.match(r"^_+ (.+?) _+$", stripped)
            if m:
                if current_test:
                    failures.append({
                        "test": current_test,
                        "output": "\n".join(current_block),
                    })
                current_test = m.group(1)
                current_block = []
            else:
                current_block.append(line)
            continue

    if current_test:
        failures.append({
            "test": current_test,
            "output": "\n".join(current_block),
        })

    # Also grab FAILED lines from the summary if no FAILURES section found.
    if not failures:
        for line in lines:
            m = re.match(r"^FAILED\s+(.+?)(?:\s+-\s+(.+))?$", line.strip())
            if m:
                failures.append({
                    "test": m.group(1),
                    "output": m.group(2) or "",
                })

    return failures


def _collect_failures_from_dir(artifact_dir: str) -> list[dict[str, str]]:
    """Scan an artifact directory for log/test files and extract failures."""
    all_failures: list[dict[str, str]] = []

    for root, _dirs, files in os.walk(artifact_dir):
        for fname in files:
            # Skip generated report files to avoid double-counting.
            if fname.endswith(FAILED_REPORT_SUFFIX) or fname.endswith(SKIPPED_REPORT_SUFFIX):
                continue

            fpath = os.path.join(root, fname)

            if fname.endswith(".txt"):
                with open(fpath, encoding="utf-8", errors="replace") as f:
                    content = f.read()
                all_failures.extend(_parse_go_test_results(content, "FAIL"))

            elif fname == "report.xml":
                all_failures.extend(_parse_junit_xml_failures(fpath))

            elif fname.endswith(".log"):
                with open(fpath, encoding="utf-8", errors="replace") as f:
                    content = f.read()
                # Try Go format first, then pytest format.
                go_failures = _parse_go_test_results(content, "FAIL")
                if go_failures:
                    all_failures.extend(go_failures)
                else:
                    all_failures.extend(_parse_pytest_log_failures(content))

    return all_failures


def _collect_skips_from_dir(artifact_dir: str) -> list[dict[str, str]]:
    """Scan an artifact directory for *.skipped.txt report files and extract skips."""
    all_skips: list[dict[str, str]] = []

    for root, _dirs, files in os.walk(artifact_dir):
        for fname in files:
            if fname.endswith(SKIPPED_REPORT_SUFFIX):
                fpath = os.path.join(root, fname)
                with open(fpath, encoding="utf-8", errors="replace") as f:
                    content = f.read()
                all_skips.extend(_parse_go_test_results(content, "SKIP"))

    return all_skips


@mcp.tool()
def get_run_failures(run_id: str) -> str:
    """Download log artifacts of a GitHub Actions run and return all test failures.

    Downloads all artifacts from the specified run, parses Go test output (.txt),
    JUnit XML (report.xml), pytest logs (.log), and *.skipped.txt reports for
    skips, and returns a summary organized by job name. Generated *.failed.txt
    and *.skipped.txt files are skipped when scanning for failures to avoid
    double-counting with the raw logs.

    Args:
        run_id: The GitHub Actions run ID (e.g. '12345678901').
    """
    lines: list[str] = []

    # Download artifacts.
    lines.append(f"Downloading artifacts for run {run_id}...")
    try:
        tmp_dir = _download_artifacts(run_id)
    except (subprocess.CalledProcessError, RuntimeError) as e:
        err = e.stderr or e.stdout if isinstance(e, subprocess.CalledProcessError) else str(e)
        return f"ERROR downloading artifacts: {err or str(e)}"

    lines.append(f"Artifacts downloaded to {tmp_dir}")

    # Scan each artifact directory.
    job_failures: dict[str, list[dict[str, str]]] = {}
    job_skips: dict[str, list[dict[str, str]]] = {}
    try:
        for entry in sorted(os.listdir(tmp_dir)):
            entry_path = os.path.join(tmp_dir, entry)
            if not os.path.isdir(entry_path):
                continue

            failures = _collect_failures_from_dir(entry_path)
            if failures:
                job_failures[entry] = failures

            skips = _collect_skips_from_dir(entry_path)
            if skips:
                job_skips[entry] = skips
    finally:
        # Clean up temp dir.
        shutil.rmtree(tmp_dir, ignore_errors=True)

    if not job_failures and not job_skips:
        lines.append("\nNo test failures or skips found in any artifact.")
        return "\n".join(lines)

    # Format failure output.
    if job_failures:
        total = sum(len(f) for f in job_failures.values())
        lines.append(f"\n{total} failure(s) across {len(job_failures)} job(s):\n")

        for job_name, failures in sorted(job_failures.items()):
            lines.append(f"{'=' * 60}")
            lines.append(f"JOB: {job_name} ({len(failures)} failure(s))")
            lines.append(f"{'=' * 60}")
            for failure in failures:
                lines.append(f"\n  --- {failure['test']} ---")
                for out_line in failure["output"].splitlines():
                    lines.append(f"  {out_line}")
            lines.append("")
    else:
        lines.append("\nNo test failures found in any artifact.")

    # Format skip output.
    if job_skips:
        total_skips = sum(len(s) for s in job_skips.values())
        lines.append(f"\n{total_skips} skip(s) across {len(job_skips)} job(s):\n")

        for job_name, skips in sorted(job_skips.items()):
            lines.append(f"{'=' * 60}")
            lines.append(f"JOB: {job_name} ({len(skips)} skip(s))")
            lines.append(f"{'=' * 60}")
            for skip in skips:
                lines.append(f"\n  --- {skip['test']} ---")
                for out_line in skip["output"].splitlines():
                    lines.append(f"  {out_line}")
            lines.append("")

    return "\n".join(lines)


@mcp.tool()
def delete_build_dev_run(run_id: str) -> str:
    """Delete a GitHub Actions Build (dev) workflow run.

    Args:
        run_id: The GitHub Actions 'Build (dev)' workflow run ID (e.g. '12345678901').
    """
    try:
        _run(["gh", "api", "--method", "DELETE", f"repos/{GH_REPO}/actions/runs/{run_id}"])
    except subprocess.CalledProcessError as e:
        return f"ERROR deleting run {run_id}: {e.stderr or e.stdout or str(e)}"

    return f"Run {run_id} deleted."


@mcp.tool()
def download_build_dev_artifacts(run_id: str, repo_dir: str) -> str:
    """Download all artifacts for a Build (dev) run into the workspace.

    Artifacts are extracted to '<repo_dir>/build-dev-artifacts/<run_id>/', with
    one subdirectory per artifact.

    Args:
        run_id: The GitHub Actions run ID (e.g. '12345678901').
        repo_dir: Absolute path to the azure-linux-image-tools git checkout.
    """
    dest_dir = os.path.join(repo_dir, "build-dev-artifacts", run_id)
    try:
        path = _download_artifacts(run_id, dest_dir=dest_dir)
    except (subprocess.CalledProcessError, RuntimeError) as e:
        err = e.stderr or e.stdout if isinstance(e, subprocess.CalledProcessError) else str(e)
        return f"ERROR downloading artifacts: {err or str(e)}"

    entries = sorted(
        e for e in os.listdir(path)
        if os.path.isdir(os.path.join(path, e))
    )
    lines = [f"Downloaded {len(entries)} artifact(s) to {path}"]
    for entry in entries:
        lines.append(f"  - {entry}")
    return "\n".join(lines)


@mcp.tool()
def download_image(distro: str, variant: str, version: str, arch: str) -> str:
    """Download a base image from the maritimusgithubstorage Azure blob cache.

    The image is downloaded into `<repo>/test/base-images/` and renamed to
    `<distro>-<variant>-<version>-<arch>.<format>`. If that file already exists, the download is skipped.

    Args:
        distro: Distro name (e.g. 'azure-linux', 'ubuntu').
        variant: Image variant (e.g. 'core-efi', 'core-legacy', 'azure-cloud').
        version: Distro version (e.g. '2.0', '3.0', '4.0', '22.04', '24.04').
        arch: Architecture ('amd64' or 'arm64').
        format: Image file extension ('vhd' or 'vhdx').
    """
    format = "vhd" if variant == "core-legacy" else "vhdx"
    output_path = os.path.join(BASE_IMAGE_DIR, f"{distro}-{variant}-{version}-{arch}.{format}")

    if os.path.isfile(output_path):
        return f"Base image already exists: {output_path}"

    original_sub = ""
    try:
        try:
            stdout = _run(["az", "account", "show", "--query", "id", "-o", "tsv"]).stdout
            original_sub = cast(str, stdout).strip()
        except subprocess.CalledProcessError as e:
            return f"ERROR reading current az subscription: {e.stderr or e.stdout or str(e)}"

        _run(["az", "account", "set", "--subscription", BASE_IMAGE_SUBSCRIPTION])

        base_image_name = f"{distro}/{variant}-{format}-{version}-{arch}"
        _run([DOWNLOAD_IMAGE_SH, BASE_IMAGE_STORAGE_ACCOUNT, BASE_IMAGE_CONTAINER, base_image_name, BASE_IMAGE_DIR])
    except subprocess.CalledProcessError as e:
        return f"ERROR downloading image: {e.stderr or e.stdout or str(e)}"
    finally:
        if original_sub:
            try:
                _run(["az", "account", "set", "--subscription", original_sub])
            except subprocess.CalledProcessError:
                pass

    src_path = os.path.join(BASE_IMAGE_DIR, f"image.{format}")
    try:
        shutil.move(src_path, output_path)
    except OSError as e:
        return f"ERROR moving {src_path} to {output_path}: {e}"

    return f"SUCCESS saved download to: {output_path}"


@mcp.tool()
def wait_build_dev(run_id: str) -> str:
    """Wait for a Build (dev) workflow run to complete.

    Blocks until the run reaches a terminal status, regardless of whether jobs
    fail along the way. Returns the final status and conclusion.

    Args:
        run_id: The GitHub Actions run ID (e.g. '12345678901').
    """
    run_url = f"https://github.com/{GH_REPO}/actions/runs/{run_id}"

    try:
        for run_json in _watch_run(run_id):
            if run_json.get("status") == "completed":
                conclusion = run_json.get("conclusion", "")
                return f"RUN COMPLETED: {conclusion}\nRun URL: {run_url}"
    except (TimeoutError, RuntimeError) as e:
        return f"ERROR waiting for run {run_id}: {e}\nRun URL: {run_url}"

    return f"RUN COMPLETED\nRun URL: {run_url}"


@mcp.tool()
def run_build_dev(
    repo_dir: str,
    run_functional_tests: bool = True,
    run_vm_tests: bool = True,
    branch: str = "",
) -> str:
    """Trigger the GitHub Actions 'Build (dev)' workflow on the current branch.

    Returns the run ID and URL as soon as the run is detected. Use
    `wait_build_dev` to wait for completion and `get_run_failures` to inspect
    failures.

    Args:
        repo_dir: Absolute path to the azure-linux-image-tools git checkout.
        run_functional_tests: Whether to run functional tests. (default: True)
        run_vm_tests: Whether to run VM tests. (default: True)
        branch: Upstream branch name (e.g. 'user/foo/my-branch'). (default: currently checked out branch's upstream)
    """
    repo = GH_REPO
    workflow = "build-dev.yml"

    if not branch:
        branch = _get_upstream(repo_dir)

    lines = [f"Triggering 'Build (dev)' on branch: {branch}"]

    # Trigger the workflow.
    cmd = [
        "gh", "workflow", "run", workflow,
        "--repo", repo, "--ref", branch,
        "-f", f"runFunctionalTests={str(run_functional_tests).lower()}",
        "-f", f"runVMTests={str(run_vm_tests).lower()}",
    ]
    _run(cmd)
    lines.append("Workflow dispatched. Waiting for run to appear...")

    # Wait for the run to register.
    time.sleep(5)

    run_id = None
    for _ in range(12):
        try:
            result = _run([
                "gh", "run", "list", "--repo", repo, "--workflow", workflow,
                "--branch", branch, "--limit", "1",
                "--json", "databaseId,status", "--jq", ".[0].databaseId",
            ])
            rid = result.stdout.strip()
            rid = cast(str, rid) # Can't be inferred, true because text=True in _run.
            if rid:
                run_id = rid
                break
        except subprocess.CalledProcessError:
            pass
        time.sleep(5)

    if not run_id:
        return "ERROR: Could not find the triggered workflow run."

    run_url = f"https://github.com/{repo}/actions/runs/{run_id}"
    lines.append(f"Run ID: {run_id}")
    lines.append(f"Run URL: {run_url}")
    lines.append("")
    lines.append("Use wait_build_dev to wait for the run to complete.")
    lines.append("Use get_run_failures to inspect test failures after completion.")

    return "\n".join(lines)


if __name__ == "__main__":
    mcp.run()
