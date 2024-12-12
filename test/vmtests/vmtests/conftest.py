# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import os
import random
import shutil
import string
import tempfile
from pathlib import Path
from typing import Generator

import docker
import pytest
from docker import DockerClient

SCRIPT_PATH = Path(__file__).parent
TEST_CONFIGS_DIR = SCRIPT_PATH.joinpath("../../../toolkit/tools/pkg/imagecustomizerlib/testdata")


def pytest_addoption(parser: pytest.Parser) -> None:
    parser.addoption("--keep-environment", action="store_true", help="Keep the resources created during the test")
    parser.addoption("--core-efi-azl2", action="store", help="Path to Azure Linux 2.0 core-efi qcow2 image")
    parser.addoption("--image-customizer-container-url", action="store", help="Image Customizer container image URL")


@pytest.fixture(scope="session")
def keep_environment(request: pytest.FixtureRequest) -> Generator[bool, None, None]:
    flag = request.config.getoption("--keep-environment")
    assert isinstance(flag, bool)
    yield flag


@pytest.fixture(scope="session")
def session_temp_dir(request: pytest.FixtureRequest, keep_environment: bool) -> Generator[Path, None, None]:
    build_dir = SCRIPT_PATH.joinpath("build")
    os.makedirs(build_dir, exist_ok=True)

    temp_path = tempfile.mkdtemp(prefix="vmtests-", dir=build_dir)

    # Ensure VM can access directory.
    os.chmod(temp_path, 0o775)

    yield Path(temp_path)

    if not keep_environment:
        shutil.rmtree(temp_path)


@pytest.fixture(scope="function")
def test_instance_name(request: pytest.FixtureRequest) -> Generator[str, None, None]:
    instance_suffix = "".join(random.choice(string.ascii_uppercase) for _ in range(5))
    yield request.node.name + "-" + instance_suffix


# pytest has an in-built fixture called tmp_path. But that uses /tmp, which sits in memory.
# That can be problematic when dealing with image files, which can be quite large.
@pytest.fixture(scope="function")
def test_temp_dir(
    request: pytest.FixtureRequest, session_temp_dir: Path, test_instance_name: str, keep_environment: bool
) -> Generator[Path, None, None]:
    temp_path = session_temp_dir.joinpath(test_instance_name)

    # Ensure VM can access directory.
    temp_path.mkdir(0o775)

    yield Path(temp_path)

    if not keep_environment:
        shutil.rmtree(temp_path)


@pytest.fixture(scope="session")
def core_efi_azl2(request: pytest.FixtureRequest) -> Generator[Path, None, None]:
    image = request.config.getoption("--core-efi-azl2")
    if not image:
        raise Exception("--core-efi-azl2 is required for test")
    yield Path(image)


@pytest.fixture(scope="session")
def image_customizer_container_url(request: pytest.FixtureRequest) -> Generator[str, None, None]:
    url = request.config.getoption("--image-customizer-container-url")
    if not url:
        raise Exception("--image-customizer-container-url is required for test")
    yield url


@pytest.fixture(scope="session")
def docker_client() -> Generator[DockerClient, None, None]:
    client = docker.from_env()
    yield client

    client.close()  # type: ignore
