# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import shutil
from pathlib import Path
from typing import Generator

import pytest

from ..conftest import (
    close_list,
    create_temp_folder,
    docker_client,
    image_customizer_container_url,
    keep_environment,
    libvirt_conn,
    libvirt_event_thread,
    logs_dir,
    ssh_key,
)


def pytest_addoption(parser: pytest.Parser) -> None:
    parser.addoption(
        "--keep-environment",
        action="store_true",
        help="Keep the resources created during the test",
    )
    parser.addoption("--input-image", action="store", help="Path to input image")
    parser.addoption("--osmodifier-binary", action="store", help="Path to osmodifier binary")
    parser.addoption("--logs-dir", action="store", help="Path to logs directory")
    parser.addoption(
        "--ssh-private-key",
        action="store",
        help="An SSH private key file to use for authentication with the VMs",
    )
    parser.addoption(
        "--image-customizer-container-url",
        action="store",
        help="Image Customizer container image URL",
    )


@pytest.fixture(scope="session")
def session_temp_dir(request: pytest.FixtureRequest, keep_environment: bool) -> Generator[Path, None, None]:
    temp_path = create_temp_folder("osmodifiertest-")
    yield Path(temp_path)

    if not keep_environment:
        shutil.rmtree(temp_path)


@pytest.fixture(scope="session")
def osmodifier_binary(request: pytest.FixtureRequest) -> Path:
    path = request.config.getoption("--osmodifier-binary")
    if not path:
        raise Exception("--osmodifier-binary is required for this test")
    return Path(path)


@pytest.fixture(scope="session")
def input_image(request: pytest.FixtureRequest) -> Generator[Path, None, None]:
    image = request.config.getoption("--input-image")
    if not image:
        raise Exception("--input-image is required for test")
    yield Path(image)
