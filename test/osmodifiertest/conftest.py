# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import random
import shutil
import string
from pathlib import Path
from typing import Generator, List

import pytest
from vmtests.vmtests.conftest import (
    close_list,
    create_temp_folder,
    docker_client,
    image_customizer_container_url,
    keep_environment,
    libvirt_conn,
    logs_dir,
    ssh_key,
    test_instance_name,
)
from vmtests.vmtests.utils.closeable import Closeable

SCRIPT_PATH = Path(__file__).parent
TEST_CONFIGS_DIR = SCRIPT_PATH.joinpath(
    "../../toolkit/tools/pkg/imagecustomizerlib/testdata"
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
    temp_path = create_temp_folder("osmodifiertests-")
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
def test_instance_name(request: pytest.FixtureRequest) -> Generator[str, None, None]:
    instance_suffix = "".join(random.choice(string.ascii_uppercase) for _ in range(5))
    yield request.node.name + "-" + instance_suffix


@pytest.fixture(scope="session")
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
