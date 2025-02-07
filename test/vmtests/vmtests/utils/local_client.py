# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

# Mostly just a wrapper around the subprocess library but can
# live log stdout and stderr, while also collecting them as
# strings.

import logging
import subprocess
import time
from io import StringIO
from pathlib import Path
from threading import Thread
from typing import IO, Any, Dict, List, Optional, Union


# The result of a subprocess execution.
class LocalExecutableResult:
    def __init__(
        self,
        stdout: str,
        stderr: str,
        exit_code: int,
        cmd: Union[str, List[str]],
        elapsed: float,
        is_timeout: bool,
    ) -> None:
        self.stdout = stdout
        self.stderr = stderr
        self.exit_code = exit_code
        self.cmd = cmd
        self.elapsed = elapsed
        self.is_timeout = is_timeout

    def check_exit_code(self) -> None:
        if self.is_timeout:
            raise Exception("Process timed out")

        elif self.exit_code != 0:
            raise Exception(f"Process failed with exit code: {self.exit_code}\ncmd=({self.cmd})")


# Handles reading a pipe (stdout or stderr).
# The contents are both collected as a string and logged.
class _PipeReader:
    def __init__(self, pipe: IO[str], log_level: int, log_name: str) -> None:
        self._pipe = pipe
        self._log_level = log_level
        self._log_name = log_name
        self._output: Optional[str] = None

        self._thread: Thread = Thread(target=self._read_thread)
        self._thread.start()

    def close(self) -> None:
        self._thread.join()

    def __enter__(self) -> "_PipeReader":
        return self

    def __exit__(self, exc_type: Any, exc_value: Any, traceback: Any) -> None:
        self.close()

    def wait_for_output(self) -> str:
        self._thread.join()

        assert self._output is not None
        return self._output

    def _read_thread(self) -> None:
        log_enabled = logging.getLogger().isEnabledFor(self._log_level)

        with StringIO() as output:
            while True:
                # Read output one line at a time.
                line = self._pipe.readline()
                if not line:
                    break

                # Store the line.
                output.write(line)

                # Log the line.
                if log_enabled:
                    line_strip_newline = line[:-1] if line.endswith("\n") else line
                    logging.log(self._log_level, "%s: %s", self._log_name, line_strip_newline)

            self._pipe.close()
            self._output = output.getvalue()


class LocalProcess:
    def __init__(
        self,
        cmd: Union[str, List[str]],
        proc: subprocess.Popen[str],
        stdout_log_level: int,
        stderr_log_level: int,
    ) -> None:
        self.cmd = cmd
        self._proc = proc
        self._result: Optional[LocalExecutableResult] = None

        self._start_time = time.monotonic()

        logging.debug("[%d][cmd]: %s", proc.pid, cmd)

        assert proc.stdout
        assert proc.stderr

        self._stdout_reader = _PipeReader(proc.stdout, stdout_log_level, f"[{proc.pid}][stdout]")
        self._stderr_reader = _PipeReader(proc.stderr, stderr_log_level, f"[{proc.pid}][stderr]")

    def close(self) -> None:
        self._proc.kill()
        self._stdout_reader.close()
        self._stderr_reader.close()

    def __enter__(self) -> "LocalProcess":
        return self

    def __exit__(self, exc_type: Any, exc_value: Any, traceback: Any) -> None:
        self.close()

    # Wait for the subprocess to exit.
    # If the timeout expires, the subprocess is killed.
    def wait(
        self,
        timeout: float = 600,
    ) -> LocalExecutableResult:
        result = self._result
        if result is None:
            # Wait for the process to exit.
            completed = False
            try:
                exit_code = self._proc.wait(timeout)
                completed = True

            except subprocess.TimeoutExpired:
                self._proc.kill()
                exit_code = self._proc.wait()

            # Get the process's output.
            stdout = self._stdout_reader.wait_for_output()
            stderr = self._stderr_reader.wait_for_output()

            elapsed_time = time.monotonic() - self._start_time

            logging.debug("[%d][cmd]: execution time: %f, exit code: %d", self._proc.pid, elapsed_time, exit_code)

            result = LocalExecutableResult(stdout, stderr, exit_code, self.cmd, elapsed_time, not completed)
            self._result = result

        return result


# Runs a subprocess and wait for the result.
# Stdout and stderr are both logged and collected as strings that are returned as part of the result.
def run(
    cmd: Union[str, List[str]],
    shell: bool = False,
    cwd: Optional[Path] = None,
    env: Optional[Dict[str, str]] = None,
    stdout_log_level: int = logging.DEBUG,
    stderr_log_level: int = logging.DEBUG,
    timeout: float = 600,
) -> LocalExecutableResult:
    with popen(
        cmd,
        shell=shell,
        cwd=cwd,
        env=env,
        stdout_log_level=stdout_log_level,
        stderr_log_level=stderr_log_level,
    ) as process:
        return process.wait(
            timeout=timeout,
        )


# Runs a subprocess.
# Stdout and stderr are both logged and collected as strings that are returned as part of the result.
def popen(
    cmd: Union[str, List[str]],
    shell: bool = False,
    cwd: Optional[Path] = None,
    env: Optional[Dict[str, str]] = None,
    stdout_log_level: int = logging.DEBUG,
    stderr_log_level: int = logging.DEBUG,
) -> LocalProcess:
    proc = subprocess.Popen(
        cmd, shell=shell, env=env, cwd=cwd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, encoding="utf-8"
    )
    return LocalProcess(cmd, proc, stdout_log_level, stderr_log_level)
