# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

from pathlib import Path
from typing import Generator, List

import pytest


def pytest_addoption(parser: pytest.Parser) -> None:
    parser.addoption("--image-creator-binary-path", action="store", help="Path to Image Creator binary")
    parser.addoption("--rpm-sources", action="append", help="Path to RPM source files or directories")
    parser.addoption("--tools-tar", action="store", help="Path to tools tar file")
    parser.addoption("--image-customizer-binary-path", action="store", help="Path to Image Customizer binary")


@pytest.fixture(scope="session")
def image_creator_binary_path(request: pytest.FixtureRequest) -> Generator[Path, None, None]:
    binary_path = request.config.getoption("--image-creator-binary-path")
    if not binary_path:
        raise Exception("--image-creator-binary-path is required for imagecreator tests")
    yield Path(binary_path)


@pytest.fixture(scope="session")
def rpm_sources(request: pytest.FixtureRequest) -> Generator[List[Path], None, None]:
    sources = request.config.getoption("--rpm-sources")
    if not sources:
        raise Exception("--rpm-sources is required for test")
    yield [Path(source) for source in sources]


@pytest.fixture(scope="session")
def tools_tar(request: pytest.FixtureRequest) -> Generator[Path, None, None]:
    tar_path = request.config.getoption("--tools-tar")
    if not tar_path:
        raise Exception("--tools-tar is required for test")
    yield Path(tar_path)


@pytest.fixture(scope="session")
def image_customizer_binary_path(request: pytest.FixtureRequest) -> Generator[Path, None, None]:
    binary_path = request.config.getoption("--image-customizer-binary-path")
    if not binary_path:
        raise Exception("--image-customizer-binary-path is required for test")
    yield Path(binary_path)
