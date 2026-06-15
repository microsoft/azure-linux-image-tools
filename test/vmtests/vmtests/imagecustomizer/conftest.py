# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

from pathlib import Path
from typing import Generator

import pytest


def pytest_addoption(parser: pytest.Parser) -> None:
    # customize subcommand test options (Azure Linux 2/3/4)
    parser.addoption("--core-efi-azl2", action="store", help="Path to Azure Linux 2 core EFI image")
    parser.addoption("--core-efi-azl3", action="store", help="Path to Azure Linux 3 core EFI image")
    parser.addoption("--core-efi-azl4", action="store", help="Path to Azure Linux 4 core EFI image")
    parser.addoption("--core-legacy-azl2", action="store", help="Path to Azure Linux 2 core legacy image")
    parser.addoption("--core-legacy-azl3", action="store", help="Path to Azure Linux 3 core legacy image")
    parser.addoption("--core-legacy-azl4", action="store", help="Path to Azure Linux 4 core legacy image")

    # create subcommand test options (Azure Linux 3/4 and Fedora 42)
    parser.addoption("--rpm-sources-azl3", action="store", help="Path to Azure Linux 3 RPM sources directory")
    parser.addoption("--rpm-sources-azl4", action="store", help="Path to Azure Linux 4 RPM sources directory")
    parser.addoption("--rpm-sources-fedora42", action="store", help="Path to Fedora 42 RPM sources directory")
    parser.addoption("--tools-dir-azl3", action="store", help="Path to Azure Linux 3 tools directory")
    parser.addoption("--tools-dir-azl4", action="store", help="Path to Azure Linux 4 tools directory")
    parser.addoption("--tools-dir-fedora42", action="store", help="Path to Fedora 42 tools directory")


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
def core_efi_azl4(request: pytest.FixtureRequest) -> Generator[Path, None, None]:
    image = request.config.getoption("--core-efi-azl4")
    if not image:
        pytest.skip("--core-efi-azl4 is required for test")
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


@pytest.fixture(scope="session")
def core_legacy_azl4(request: pytest.FixtureRequest) -> Generator[Path, None, None]:
    image = request.config.getoption("--core-legacy-azl4")
    if not image:
        pytest.skip("--core-legacy-azl4 is required for test")
    yield Path(image)


@pytest.fixture(scope="session")
def rpm_sources_azl3(request: pytest.FixtureRequest) -> Generator[Path, None, None]:
    source = request.config.getoption("--rpm-sources-azl3")
    if not source:
        pytest.skip("--rpm-sources-azl3 is required for test")
    yield Path(source)


@pytest.fixture(scope="session")
def rpm_sources_azl4(request: pytest.FixtureRequest) -> Generator[Path, None, None]:
    source = request.config.getoption("--rpm-sources-azl4")
    if not source:
        pytest.skip("--rpm-sources-azl4 is required for test")
    yield Path(source)


@pytest.fixture(scope="session")
def rpm_sources_fedora42(request: pytest.FixtureRequest) -> Generator[Path, None, None]:
    source = request.config.getoption("--rpm-sources-fedora42")
    if not source:
        pytest.skip("--rpm-sources-fedora42 is required for test")
    yield Path(source)


@pytest.fixture(scope="session")
def tools_dir_azl3(request: pytest.FixtureRequest) -> Generator[Path, None, None]:
    dir_path = request.config.getoption("--tools-dir-azl3")
    if not dir_path:
        pytest.skip("--tools-dir-azl3 is required for test")
    yield Path(dir_path)


@pytest.fixture(scope="session")
def tools_dir_azl4(request: pytest.FixtureRequest) -> Generator[Path, None, None]:
    dir_path = request.config.getoption("--tools-dir-azl4")
    if not dir_path:
        pytest.skip("--tools-dir-azl4 is required for test")
    yield Path(dir_path)


@pytest.fixture(scope="session")
def tools_dir_fedora42(request: pytest.FixtureRequest) -> Generator[Path, None, None]:
    dir_path = request.config.getoption("--tools-dir-fedora42")
    if not dir_path:
        pytest.skip("--tools-dir-fedora42 is required for test")
    yield Path(dir_path)
