# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

from docker import DockerClient

from ..utils.docker_utils import container_run


# Ensure the --help command can run without the container being privileged.
def test_container_help(
    docker_client: DockerClient,
    image_customizer_container_url: str,
) -> None:
    _, _ = container_run(
        docker_client,
        image_customizer_container_url,
        ["--help"],
        detach=True,
        privileged=False,
    )


# Ensure the --version command can run without the container being privileged.
def test_container_version(
    docker_client: DockerClient,
    image_customizer_container_url: str,
) -> None:
    _, _ = container_run(
        docker_client,
        image_customizer_container_url,
        ["--version"],
        detach=True,
        privileged=False,
    )
