# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import random
import shutil
import string
from pathlib import Path
from typing import Generator

import pytest

from ..conftest import create_temp_folder


def pytest_addoption(parser: pytest.Parser) -> None:
    parser.addoption("--keep-environment", action="store_true", help="Keep the resources created during the test")
    parser.addoption("--core-efi-azl2", action="store", help="Path to input image")
    parser.addoption("--core-efi-azl3", action="store", help="Path to input image")
    parser.addoption("--core-legacy-azl2", action="store", help="Path to input image")
    parser.addoption("--core-legacy-azl3", action="store", help="Path to input image")
    parser.addoption("--logs-dir", action="store", help="Path to logs directory")
    parser.addoption("--image-customizer-container-url", action="store", help="Image Customizer container image URL")
    parser.addoption(
        "--ssh-private-key", action="store", help="An SSH private key file to use for authentication with the VMs"
    )


@pytest.fixture(scope="session")
def keep_environment(request: pytest.FixtureRequest) -> Generator[bool, None, None]:
    flag = request.config.getoption("--keep-environment")
    assert isinstance(flag, bool)
    yield flag


@pytest.fixture(scope="session")
def session_temp_dir(request: pytest.FixtureRequest, keep_environment: bool) -> Generator[Path, None, None]:
    temp_path = create_temp_folder("vmtests-")
    yield Path(temp_path)

    if not keep_environment:
        shutil.rmtree(temp_path)


@pytest.fixture(scope="function")
def test_instance_name(request: pytest.FixtureRequest) -> Generator[str, None, None]:
    instance_suffix = "".join(random.choice(string.ascii_uppercase) for _ in range(5))
    yield request.node.name + "-" + instance_suffix


@pytest.fixture(scope="session")
def core_efi_azl2(request: pytest.FixtureRequest) -> Generator[Path, None, None]:
    image = request.config.getoption("--core-efi-azl2")
    if not image:
        pytest.skip("--core-efi-azl2 is required for test")
    yield Path(image)


@pytest.fixture(scope="session")
def core_efi_azl3(request: pytest.FixtureRequest) -> Generator[Path, None, None]:
    image = request.config.getoption("--core-efi-azl3")
    if not image:
        pytest.skip("--core-efi-azl3 is required for test")
    yield Path(image)


@pytest.fixture(scope="session")
def core_legacy_azl2(request: pytest.FixtureRequest) -> Generator[Path, None, None]:
    image = request.config.getoption("--core-legacy-azl2")
    if not image:
        pytest.skip("--core-legacy-azl2 is required for test")
    yield Path(image)


@pytest.fixture(scope="session")
def core_legacy_azl3(request: pytest.FixtureRequest) -> Generator[Path, None, None]:
    image = request.config.getoption("--core-legacy-azl3")
    if not image:
        pytest.skip("--core-legacy-azl3 is required for test")
    yield Path(image)
