#!/usr/bin/env python3
# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

"""Generate test failure and skip reports from a Go test output .txt file.

Usage: go-test-report.py <test-output-txt>

Given an input file <name>.txt, generates alongside it:
  <name>.failed.txt  - Failed test blocks (only written if there are failures)
  <name>.skipped.txt - Skipped test blocks (only written if there are skips)
"""

import re
import sys
from pathlib import Path


RUN_RE = re.compile(r"^=== RUN\s+(\S+)")
PARENT_RESULT_RE = re.compile(r"^--- \w+:\s+(\S+)")
FAIL_RE = re.compile(r"^\s*--- FAIL:")
SKIP_RE = re.compile(r"^\s*--- SKIP:")


def _extract_blocks(lines: list[str], marker_re: re.Pattern[str]) -> list[str]:
    """Extract Go test blocks whose result lines match marker_re.

    A block starts at a top-level '=== RUN' and includes all nested subtest
    '=== RUN' lines, body output, the parent '--- <result>:' line, and any
    indented child '--- <result>:' lines. A block is emitted when any result
    line (parent or child) matches marker_re.
    """
    blocks: list[str] = []
    buf: list[str] = []
    matched = False
    run_name = ""
    state = "idle"  # idle | in_block | in_results

    for line in lines:
        if state == "idle":
            m = RUN_RE.match(line)
            if m:
                buf = [line]
                run_name = m.group(1)
                matched = False
                state = "in_block"

        elif state == "in_block":
            buf.append(line)
            m = PARENT_RESULT_RE.match(line)
            if m:
                if m.group(1) != run_name:
                    raise ValueError(
                        f"Parent result name {m.group(1)!r} does not match "
                        f"block run name {run_name!r}: {line!r}"
                    )

                if marker_re.match(line):
                    matched = True

                state = "in_results"

        elif state == "in_results":
            m = RUN_RE.match(line)
            if m:
                # New block started; finalize current one.
                if matched:
                    blocks.append("\n".join(buf))

                buf = [line]
                run_name = m.group(1)
                matched = False
                state = "in_block"
            else:
                buf.append(line)
                if marker_re.match(line):
                    matched = True

    # Finalize any pending block at EOF.
    if state == "in_results" and matched:
        blocks.append("\n".join(buf))

    return blocks


def main() -> None:
    if len(sys.argv) != 2:
        print(f"Usage: {sys.argv[0]} <test-output-txt>", file=sys.stderr)
        sys.exit(1)

    test_output = Path(sys.argv[1])

    if test_output.suffix != ".txt":
        print(f"ERROR: Test output file must have a .txt extension: {test_output!r}", file=sys.stderr)
        sys.exit(1)

    if not test_output.is_file():
        print(f"ERROR: Test output file not found: {test_output!r}", file=sys.stderr)
        sys.exit(1)

    lines = test_output.read_text(encoding="utf-8", errors="replace").splitlines()

    failed_blocks = _extract_blocks(lines, FAIL_RE)
    skipped_blocks = _extract_blocks(lines, SKIP_RE)

    failed_report = test_output.with_suffix(".failed.txt")
    skipped_report = test_output.with_suffix(".skipped.txt")

    failed_text = "\n".join(failed_blocks) + "\n" if failed_blocks else ""
    skipped_text = "\n".join(skipped_blocks) + "\n" if skipped_blocks else ""

    # Only write report files when non-empty; otherwise remove any stale file
    # so downstream artifact uploads don't include empty placeholders.
    for report_path, text in ((failed_report, failed_text), (skipped_report, skipped_text)):
        if text:
            report_path.write_text(text, encoding="utf-8")
        elif report_path.exists():
            report_path.unlink()

    failed_bytes = len(failed_text.encode("utf-8"))
    skipped_bytes = len(skipped_text.encode("utf-8"))

    print(f"Test reports generated for {test_output}:")
    print(f"  Failed tests:  {len(failed_blocks)} ({failed_report}, {failed_bytes} bytes)")
    print(f"  Skipped tests: {len(skipped_blocks)} ({skipped_report}, {skipped_bytes} bytes)")


if __name__ == "__main__":
    main()
