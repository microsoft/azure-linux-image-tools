# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import logging
import shlex
import time
from io import StringIO
from pathlib import Path
from threading import Thread
from typing import Any, Dict, List, Optional, Union

from paramiko import AutoAddPolicy, SSHClient
from paramiko.channel import ChannelFile, ChannelStderrFile


# The result of a SSH process execution.
class SshExecutableResult:
    def __init__(
        self,
        stdout: str,
        stderr: str,
        exit_code: int,
        cmd: str,
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
            raise Exception("SSH process timed out")

        elif self.exit_code != 0:
            raise Exception(f"SSH process failed with exit code: {self.exit_code}")


# Handles reading an SSH pipe (stdout or stderr).
# The contents are both collected as a string and logged.
class _SshChannelFileReader:
    def __init__(self, channel_file: ChannelFile, log_level: int, log_name: str) -> None:
        self._channel_file = channel_file
        self._log_level = log_level
        self._log_name = log_name
        self._output: Optional[str] = None

        self._thread: Thread = Thread(target=self._read_thread)
        self._thread.start()

    def close(self) -> None:
        self._thread.join()

    def __enter__(self) -> "_SshChannelFileReader":
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
                line = self._channel_file.readline()
                if not line:
                    break

                # Store the line.
                output.write(line)

                # Log the line.
                if log_enabled:
                    line_strip_newline = line[:-1] if line.endswith("\n") else line
                    logging.log(self._log_level, "%s: %s", self._log_name, line_strip_newline)

            self._channel_file.close()
            self._output = output.getvalue()


class SshProcess:
    def __init__(
        self,
        cmd: str,
        stdout: ChannelFile,
        stderr: ChannelStderrFile,
        stdout_log_level: int,
        stderr_log_level: int,
    ) -> None:
        self.cmd = cmd
        self._channel = stdout.channel
        self._result: Optional[SshExecutableResult] = None

        self._start_time = time.monotonic()

        chanid = self._channel.chanid

        logging.debug("[ssh][%d][cmd]: %s", chanid, cmd)

        self._stdout_reader = _SshChannelFileReader(stdout, stdout_log_level, f"[ssh][{chanid}][stdout]")
        self._stderr_reader = _SshChannelFileReader(stderr, stderr_log_level, f"[ssh][{chanid}][stderr]")

    def close(self) -> None:
        self._channel.close()
        self._stdout_reader.close()
        self._stderr_reader.close()

    def __enter__(self) -> "SshProcess":
        return self

    def __exit__(self, exc_type: Any, exc_value: Any, traceback: Any) -> None:
        self.close()

    def wait(
        self,
        timeout: float = 600,
    ) -> SshExecutableResult:
        result = self._result
        if result is None:
            # Wait for the process to exit.
            completed = self._channel.status_event.wait(timeout)

            if completed:
                exit_code = self._channel.recv_exit_status()

            else:
                # Close channel.
                self._channel.close()

                exit_code = 1

            # Get the process's output.
            stdout = self._stdout_reader.wait_for_output()
            stderr = self._stderr_reader.wait_for_output()

            elapsed_time = time.monotonic() - self._start_time

            logging.debug(
                "[ssh][%d][cmd]: execution time: %f, exit code: %d", self._channel.chanid, elapsed_time, exit_code
            )

            result = SshExecutableResult(stdout, stderr, exit_code, self.cmd, elapsed_time, not completed)
            self._result = result

        return result


class SshClient:
    def __init__(
        self,
        hostname: str,
        port: int = 22,
        username: Optional[str] = None,
        key_path: Optional[Path] = None,
        gateway: "Optional[SshClient]" = None,
        known_hosts_path: Optional[Path] = None,
    ) -> None:
        self.ssh_client: SSHClient

        # Handle gateway.
        # (That is, proxying an SSH connection through another SSH connection.)
        sock = None
        if gateway:
            gateway_transport = gateway.ssh_client.get_transport()
            assert gateway_transport
            sock = gateway_transport.open_channel("direct-tcpip", (hostname, port), ("", 0))

        self.ssh_client = SSHClient()

        # Handle known hosts.
        self.ssh_client.set_missing_host_key_policy(AutoAddPolicy)
        if known_hosts_path:
            self.ssh_client.load_host_keys(str(known_hosts_path))
        else:
            self.ssh_client.load_system_host_keys()

        key_filename = None if key_path is None else str(key_path.absolute())

        # Open SSH connection.
        self.ssh_client.connect(hostname=hostname, port=port, username=username, key_filename=key_filename, sock=sock)

    def close(self) -> None:
        self.ssh_client.close()

    def __enter__(self) -> "SshClient":
        return self

    def __exit__(self, exc_type: Any, exc_value: Any, traceback: Any) -> None:
        self.close()

    def run(
        self,
        cmd: str,
        shell: bool = False,
        cwd: Optional[Path] = None,
        env: Optional[Dict[str, str]] = None,
        stdout_log_level: int = logging.DEBUG,
        stderr_log_level: int = logging.DEBUG,
        timeout: float = 600,
    ) -> SshExecutableResult:
        with self.popen(
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

    def popen(
        self,
        cmd: Union[str, List[str]],
        shell: bool = False,
        cwd: Optional[Path] = None,
        env: Optional[Dict[str, str]] = None,
        stdout_log_level: int = logging.DEBUG,
        stderr_log_level: int = logging.DEBUG,
    ) -> SshProcess:
        if isinstance(cmd, list):
            cmd = shlex.join(cmd)

        elif not shell:
            # SSH runs all commands in shell sessions.
            # So, to remove shell symantics, use shlex to escape all the shell symbols.
            cmd = shlex.join(shlex.split(cmd))

        if cwd is not None:
            cmd = f"cd {shlex.quote(str(cwd))}; {cmd}"

        stdin, stdout, stderr = self.ssh_client.exec_command(cmd, environment=env)
        stdin.close()

        return SshProcess(cmd, stdout, stderr, stdout_log_level, stderr_log_level)

    def put_file(self, local_path: Path, node_path: Path) -> None:
        with self.ssh_client.open_sftp() as sftp:
            sftp.put(str(local_path), str(node_path))

    def get_file(self, node_path: Path, local_path: Path) -> None:
        with self.ssh_client.open_sftp() as sftp:
            sftp.get(str(node_path), str(local_path))
