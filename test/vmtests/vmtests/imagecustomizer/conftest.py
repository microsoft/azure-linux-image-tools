# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import shutil
from pathlib import Path
from typing import Generator

import pytest

from ..conftest import create_temp_folder


def pytest_addoption(parser: pytest.Parser) -> None:
    parser.addoption("--core-efi-azl2", action="store", help="Path to input image")
    parser.addoption("--core-efi-azl3", action="store", help="Path to input image")
    parser.addoption("--core-legacy-azl2", action="store", help="Path to input image")
    parser.addoption("--core-legacy-azl3", action="store", help="Path to input image")


@pytest.fixture(scope="session")
def session_temp_dir(request: pytest.FixtureRequest, keep_environment: bool) -> Generator[Path, None, None]:
    temp_path = create_temp_folder("vmtests-")
    yield Path(temp_path)

    if not keep_environment:
        shutil.rmtree(temp_path)


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
