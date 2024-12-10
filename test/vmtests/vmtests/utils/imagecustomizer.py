# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

from pathlib import Path

from docker import DockerClient

from .docker_utils import container_run


# Run the containerized version of the imagecustomizer tool.
def run_image_customizer(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    base_image_path: Path,
    config_path: Path,
    output_image_format: str,
    output_image_path: Path,
) -> None:
    container_base_image_dir = Path("/mic/base_image")
    container_config_dir = Path("/mic/config")
    container_output_image_dir = Path("/mic/output_image")
    container_build_dir = Path("/mic/build")

    base_image_dir = base_image_path.parent.absolute()
    config_dir = config_path.parent.absolute()
    output_image_dir = output_image_path.parent.absolute()

    container_base_image_path = container_base_image_dir.joinpath(base_image_path.name)
    container_config_path = container_config_dir.joinpath(config_path.name)
    container_output_image_path = container_output_image_dir.joinpath(output_image_path.name)

    args = [
        "imagecustomizer",
        "--image-file",
        str(container_base_image_path),
        "--config-file",
        str(container_config_path),
        "--build-dir",
        str(container_build_dir),
        "--output-image-format",
        output_image_format,
        "--output-image-file",
        str(container_output_image_path),
        "--log-level",
        "debug",
    ]

    volumes = [
        f"{base_image_dir}:{container_base_image_dir}:z",
        f"{config_dir}:{container_config_dir}:z",
        f"{output_image_dir}:{container_output_image_dir}:z",
        "/dev:/dev",
    ]

    container_run(docker_client, image_customizer_container_url, args, detach=True, privileged=True, volumes=volumes)
