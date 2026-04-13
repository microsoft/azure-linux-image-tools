# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import logging
from concurrent.futures import ThreadPoolExecutor
from typing import Any, List, Tuple

from docker import DockerClient
from docker.models.containers import Container
from docker.types.daemon import CancellableStream


# Can be used with a `with` statement for deleting a Docker container.
class ContainerRemove:
    def __init__(self, container: Container):
        self.container: Container = container

    def close(self) -> None:
        self.container.remove(force=True)

    def __enter__(self) -> "ContainerRemove":
        return self

    def __exit__(self, exc_type: Any, exc_value: Any, traceback: Any) -> None:
        self.close()


# Run a container, log its stdout and stderr, and wait for it to complete.
def container_run(docker_client: DockerClient, *args: Any, **kwargs: Any) -> Tuple[List[str], List[str]]:
    with ContainerRemove(docker_client.containers.run(*args, **kwargs)) as container:
        return container_log_and_wait(container.container)


# Waits for a docker container to exit, while logging stdout and stderr.
def container_log_and_wait(container: Container) -> Tuple[List[str], List[str]]:
    # Log stdout and stderr.
    with ThreadPoolExecutor(max_workers=2) as executor:
        stdout_work = executor.submit(_process_logs, container.logs(stdout=True, stderr=False, stream=True))
        stderr_work = executor.submit(_process_logs, container.logs(stdout=False, stderr=True, stream=True))

        stdout_lines = stdout_work.result()
        stderr_lines = stderr_work.result()

    # Wait for the container to close.
    result = container.wait()

    exit_code = result["StatusCode"]
    if exit_code != 0:
        raise Exception(f"Container failed with {exit_code}")

    return (stdout_lines, stderr_lines)


def _process_logs(logs: "CancellableStream[bytes]") -> List[str]:
    lines = []

    for log in logs:
        log_str = log.decode("utf-8", errors="replace")
        lines.append(log_str)
        logging.debug(log_str.rstrip())

    return lines
