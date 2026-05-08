# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import json
from datetime import datetime
from pathlib import Path

from docker import DockerClient

from ..conftest import TEST_CONFIGS_DIR
from ..utils.imagecustomizer import run_image_customizer


def _run_json_logs_nochange_test(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    input_image: Path,
    test_temp_dir: Path,
) -> None:
    output_format = "qcow2"
    output_image_path = test_temp_dir.joinpath("image." + output_format)

    config_path = TEST_CONFIGS_DIR.joinpath("nochange-config.yaml")

    _, stderr_lines = run_image_customizer(
        docker_client,
        image_customizer_container_url,
        "customize",
        config_path,
        output_format,
        output_image_path,
        image_file=input_image,
        log_format="json",
    )

    json_lines: list[dict[str, str]] = []

    # Ensure all the stderr lines are valid JSON.
    for line in stderr_lines:
        line_json: dict[str, str] = json.loads(line)
        json_lines.append(line_json)

        assert isinstance(line_json, dict)
        assert line_json.get("level") in ["panic", "fatal", "error", "warn", "info", "debug", "trace"]
        timestamp = line_json.get("time")
        assert isinstance(timestamp, str)
        datetime.fromisoformat(timestamp)
        assert line_json.get("msg")

    assert json_lines[-1].get("msg") == "Success!"


def test_json_logs_nochange_azl3(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    core_efi_azl3: Path,
    test_temp_dir: Path,
) -> None:
    _run_json_logs_nochange_test(docker_client, image_customizer_container_url, core_efi_azl3, test_temp_dir)


def test_json_logs_nochange_azl4(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    core_efi_azl4: Path,
    test_temp_dir: Path,
) -> None:
    _run_json_logs_nochange_test(docker_client, image_customizer_container_url, core_efi_azl4, test_temp_dir)
