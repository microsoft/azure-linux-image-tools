# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import logging
import platform
from pathlib import Path
from typing import List, Tuple

import libvirt  # type: ignore
import pytest

from ..utils.closeable import Closeable
from .test_imagecreator import IMAGECREATOR_TEST_CONFIGS_DIR, run_image_creator_test


@pytest.mark.skipif(platform.machine() != "x86_64", reason="arm64 is not supported for this combination")
def test_create_image_efi_qcow_output_fedora(
    image_creator_binary_path: Path,
    rpm_sources: List[Path],
    tools_tar: Path,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
    logs_dir: Path,
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
    image_customizer_binary_path: Path,
) -> None:
    config_path = IMAGECREATOR_TEST_CONFIGS_DIR.joinpath("fedora.yaml")
    output_format = "qcow2"

    # debug message
    logging.debug("Running test_create_image_efi_qcow_output_fedora")

    run_image_creator_test(
        image_creator_binary_path,
        rpm_sources,
        tools_tar,
        config_path,
        output_format,
        ssh_key,
        test_temp_dir,
        test_instance_name,
        logs_dir,
        libvirt_conn,
        close_list,
        image_customizer_binary_path,
        "fedora",
        "42",
    )
