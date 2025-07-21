# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.


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
from vmtests.vmtests.conftest import keep_environment, test_instance_name
from vmtests.vmtests.utils.closeable import Closeable

SCRIPT_PATH = Path(__file__).parent
TEST_CONFIGS_DIR = SCRIPT_PATH.joinpath(
    "../../toolkit/tools/pkg/imagecustomizerlib/testdata"
)
OSMODIFIER_TEST_CONFIGS_DIR = SCRIPT_PATH.joinpath(
    "../../toolkit/tools/pkg/imagecustomizerlib/testdata/osmodifiertestdata"
)


def pytest_addoption(parser: pytest.Parser) -> None:
    parser.addoption(
        "--keep-environment",
        action="store_true",
        help="Keep the resources created during the test",
    )
    parser.addoption("--input-image", action="store", help="Path to input image")
    parser.addoption(
        "--osmodifier-binary", action="store", help="Path to osmodifier binary"
    )
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
def session_temp_dir(
    request: pytest.FixtureRequest, keep_environment: bool
) -> Generator[Path, None, None]:
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
    request: pytest.FixtureRequest,
    session_temp_dir: Path,
    test_instance_name: str,
    keep_environment: bool,
) -> Generator[Path, None, None]:
    temp_path = session_temp_dir.joinpath(test_instance_name)

    # Ensure VM can access directory.
    temp_path.mkdir(0o775)

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


@pytest.fixture(scope="session")
def logs_dir(request: pytest.FixtureRequest) -> Generator[Path, None, None]:
    logs_dir = request.config.getoption("--logs-dir")
    if not logs_dir:
        logs_dir = create_temp_folder("logs-")
    yield Path(logs_dir)


@pytest.fixture(scope="session")
def image_customizer_container_url(
    request: pytest.FixtureRequest,
) -> Generator[str, None, None]:
    url = request.config.getoption("--image-customizer-container-url")
    if not url:
        raise Exception("--image-customizer-container-url is required for test")
    yield url


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

    ssh_public_key_path = ssh_private_key_path.with_name(
        ssh_private_key_path.name + ".pub"
    )
    ssh_public_key = ssh_public_key_path.read_text()
    ssh_public_key = ssh_public_key.strip()

    yield (ssh_public_key, ssh_private_key_path)


@pytest.fixture(scope="session")
def libvirt_conn() -> Generator[libvirt.virConnect, None, None]:
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
        raise ExceptionGroup("failed to close resources", exceptions)


def create_temp_folder(
    prefix: str,
) -> str:
    build_dir = SCRIPT_PATH.joinpath("build")
    os.makedirs(build_dir, exist_ok=True)

    temp_path = tempfile.mkdtemp(prefix=prefix, dir=build_dir)

    # Ensure VM can access directory.
    os.chmod(temp_path, 0o775)

    return temp_path
