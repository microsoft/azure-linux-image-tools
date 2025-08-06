# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

from pathlib import Path
from typing import Generator

import pytest


def pytest_addoption(parser: pytest.Parser) -> None:
    parser.addoption("--input-image", action="store", help="Path to input image")
    parser.addoption("--osmodifier-binary", action="store", help="Path to osmodifier binary")


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
