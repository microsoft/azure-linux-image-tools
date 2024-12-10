# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

from pathlib import Path

from docker import DockerClient

from .conftest import TEST_CONFIGS_DIR
from .utils.imagecustomizer import run_image_customizer


def test_no_change(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    core_efi_azl2: Path,
    test_temp_dir: Path,
) -> None:
    config_path = TEST_CONFIGS_DIR.joinpath("nochange-config.yaml")
    output_image_path = test_temp_dir.joinpath("image.qcow2")

    run_image_customizer(
        docker_client,
        image_customizer_container_url,
        core_efi_azl2,
        config_path,
        "qcow2",
        output_image_path,
    )
