import logging

import docker
from docker.models.containers import Container


def container_log_and_wait(container: Container) -> "docker._types.WaitContainerResponse":
    # Log stdout and stderr.
    logs = container.logs(stdout=True, stderr=True, stream=True)
    for log in logs:
        logging.debug(log.decode("utf-8").strip())

    # Wait for the container to close.
    result = container.wait()

    # Remove the container.
    # Note: We can't use auto_remove=True, since the wait() above fails if the container is deleted too quickly.
    container.remove()

    exit_code = result["StatusCode"]
    if exit_code != 0:
        raise Exception(f"Container failed with {exit_code}")

    return result
