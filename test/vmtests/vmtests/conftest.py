# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import logging
import os
import random
import shutil
import string
import tempfile
from pathlib import Path
from typing import Generator, List, Tuple

import docker
import libvirt  # type: ignore
import pytest
from docker import DockerClient

from .utils import libvirt_events_thread
from .utils.closeable import Closeable

SCRIPT_PATH = Path(__file__).parent
TEST_CONFIGS_DIR = SCRIPT_PATH.joinpath("../../../toolkit/tools/pkg/imagecustomizerlib/testdata")


def pytest_addoption(parser: pytest.Parser) -> None:
    parser.addoption("--keep-environment", action="store_true", help="Keep the resources created during the test")
    parser.addoption("--core-efi-azl2", action="store", help="Path to input image")
    parser.addoption("--core-efi-azl3", action="store", help="Path to input image")
    parser.addoption("--core-legacy-azl2", action="store", help="Path to input image")
    parser.addoption("--core-legacy-azl3", action="store", help="Path to input image")
    parser.addoption("--logs-dir", action="store", help="Path to logs directory")
    parser.addoption("--image-customizer-container-url", action="store", help="Image Customizer container image URL")
    parser.addoption("--image-creator-binary-path", action="store", help="Path to Image Creator binary")
    parser.addoption("--rpm-sources", action="append", help="Path to RPM source files or directories")
    parser.addoption("--tools-tar", action="store", help="Path to tools tar file")
    parser.addoption(
        "--ssh-private-key", action="store", help="An SSH private key file to use for authentication with the VMs"
    )
    parser.addoption("--image-customizer-binary-path", action="store", help="Path to Image Customizer binary")


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


@pytest.fixture(scope="session")
def logs_dir(request: pytest.FixtureRequest) -> Generator[Path, None, None]:
    logs_dir = request.config.getoption("--logs-dir")
    if not logs_dir:
        logs_dir = create_temp_folder("logs-")
    yield Path(logs_dir)


@pytest.fixture(scope="session")
def image_customizer_container_url(request: pytest.FixtureRequest) -> Generator[str, None, None]:
    url = request.config.getoption("--image-customizer-container-url")
    if not url:
        raise Exception("--image-customizer-container-url is required for test")
    yield url


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


@pytest.fixture(scope="session")
def docker_client() -> Generator[DockerClient, None, None]:
    client = docker.from_env()
    yield client

    client.close()  # type: ignore


@pytest.fixture(scope="session")
def ssh_key(request: pytest.FixtureRequest) -> Generator[Tuple[str, Path], None, None]:
    ssh_private_key_path_str = request.config.getoption("--ssh-private-key")
    if not ssh_private_key_path_str:
        raise Exception("--ssh-private-key is required for test")

    ssh_private_key_path = Path(ssh_private_key_path_str)

    ssh_public_key_path = ssh_private_key_path.with_name(ssh_private_key_path.name + ".pub")
    ssh_public_key = ssh_public_key_path.read_text()
    ssh_public_key = ssh_public_key.strip()

    yield (ssh_public_key, ssh_private_key_path)


@pytest.fixture(scope="session")
def libvirt_event_thread() -> Generator[None, None, None]:
    # The libvirtaio library's logs are a little spammy. So, back them off a bit.
    logging.getLogger("virEventAsyncIOImpl").setLevel(logging.INFO)
    libvirt_events_thread.init()
    yield


@pytest.fixture(scope="session")
def libvirt_conn(libvirt_event_thread: pytest.FixtureRequest) -> Generator[libvirt.virConnect, None, None]:
    # Connect to libvirt.
    libvirt_conn_str = f"qemu:///system"
    libvirt_conn = libvirt.open(libvirt_conn_str)

    yield libvirt_conn

    libvirt_conn.close()


# Fixture that will close resources after a test has run, so long as the '--keep-environment' flag is not specified.
@pytest.fixture(scope="function")
def close_list(keep_environment: bool) -> Generator[List[Closeable], None, None]:
    vm_delete_list: List[Closeable] = []

    yield vm_delete_list

    if keep_environment:
        return

    exceptions = []
    for vm in reversed(vm_delete_list):
        try:
            vm.close()
        except Exception as ex:
            exceptions.append(ex)

    if len(exceptions) > 0:
        return ExceptionGroup("failed to close resources", exceptions)


def create_temp_folder(
    prefix: str,
) -> str:
    build_dir = SCRIPT_PATH.joinpath("build")
    os.makedirs(build_dir, exist_ok=True)

    temp_path = tempfile.mkdtemp(prefix=prefix, dir=build_dir)

    # Ensure VM can access directory.
    os.chmod(temp_path, 0o775)

    return temp_path
