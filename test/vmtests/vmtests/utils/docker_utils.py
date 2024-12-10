# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import logging
from typing import Any

import docker
from docker import DockerClient
from docker.models.containers import Container


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
def container_run(docker_client: DockerClient, *args: Any, **kwargs: Any) -> "docker._types.WaitContainerResponse":
    with ContainerRemove(docker_client.containers.run(*args, **kwargs)) as container:
        return container_log_and_wait(container.container)


# Waits for a docker container to exit, while logging stdout and stderr.
def container_log_and_wait(container: Container) -> "docker._types.WaitContainerResponse":
    # Log stdout and stderr.
    logs = container.logs(stdout=True, stderr=True, stream=True)
    for log in logs:
        logging.debug(log.decode("utf-8").strip())

    # Wait for the container to close.
    result = container.wait()

    exit_code = result["StatusCode"]
    if exit_code != 0:
        raise Exception(f"Container failed with {exit_code}")

    return result
