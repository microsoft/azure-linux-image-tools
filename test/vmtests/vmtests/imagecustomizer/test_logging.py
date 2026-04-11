# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import json
from datetime import datetime
from pathlib import Path
from typing import List

from docker import DockerClient

from ..conftest import TEST_CONFIGS_DIR
from ..utils.closeable import Closeable
from ..utils.imagecustomizer import run_image_customizer


def test_json_logs_nochange(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    core_efi_azl3: Path,
    test_temp_dir: Path,
    test_instance_name: str,
    logs_dir: Path,
    close_list: List[Closeable],
) -> None:
    input_image = core_efi_azl3

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

    json_lines = []

    # Ensure all the stderr lines are valid JSON.
    for line in stderr_lines:
        line_json = json.loads(line)
        json_lines.append(line_json)

        assert isinstance(line_json, dict)
        assert line_json.get("level") in ["panic", "fatal", "error", "warn", "info", "debug", "trace"]
        timestamp = line_json.get("time")
        assert isinstance(timestamp, str)
        datetime.fromisoformat(timestamp)
        assert line_json.get("msg")

    assert json_lines[-1].get("msg") == "Success!"
