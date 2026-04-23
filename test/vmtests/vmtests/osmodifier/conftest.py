# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

from pathlib import Path
from typing import Generator

import pytest


def pytest_addoption(parser: pytest.Parser) -> None:
    parser.addoption("--input-image", action="store", help="Path to input image")
    parser.addoption("--osmodifier-binary", action="store", help="Path to osmodifier binary")
    parser.addoption("--distro-id", action="store", help="Distro ID of the input image (e.g. 'azurelinux')")
    parser.addoption("--version-id", action="store", help="Version ID of the input image (e.g. '4.0')")


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


@pytest.fixture(scope="session")
def distro_id(request: pytest.FixtureRequest) -> str:
    value = request.config.getoption("--distro-id")
    if not value:
        raise Exception("--distro-id is required for test")
    return value


@pytest.fixture(scope="session")
def version_id(request: pytest.FixtureRequest) -> str:
    value = request.config.getoption("--version-id")
    if not value:
        raise Exception("--version-id is required for test")
    return value
