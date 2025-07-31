# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import logging
import os
import platform
from pathlib import Path
from typing import List, Tuple

import libvirt  # type: ignore
import pytest
from docker import DockerClient

from .conftest import TEST_CONFIGS_DIR
from .utils import local_client
from .utils.closeable import Closeable
from .utils.host_utils import get_host_distro
from .utils.imagecustomizer import run_image_customizer
from .utils.libvirt_utils import VmSpec, create_libvirt_domain_xml
from .utils.libvirt_vm import LibvirtVm
from .utils.ssh_client import SshClient
from .utils.user_utils import get_username


def run_min_change_test(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    input_image: Path,
    input_image_azl_release: int,
    config_path: Path,
    output_format: str,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
    logs_dir: Path,
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
) -> None:

    (ssh_public_key, ssh_private_key_path) = ssh_key

    secure_boot = False

    source_boot_type = "efi"
    if Path(input_image).suffix.lower() == ".vhd":
        source_boot_type = "legacy"

    target_boot_type = source_boot_type
    if output_format == "iso":
        target_boot_type = "efi"

    output_image_path = test_temp_dir.joinpath("image." + output_format)

    logging.debug(f"Test parameters:")
    logging.debug(f"- input_image             = {input_image}")
    logging.debug(f"- input_image_azl_release = {input_image_azl_release}")
    logging.debug(f"- config_path             = {config_path}")
    logging.debug(f"- output_format           = {output_format}")
    logging.debug(f"- source_boot_type        = {source_boot_type}")
    logging.debug(f"- target_boot_type        = {target_boot_type}")
    logging.debug(f"- logs_dir                = {logs_dir}")

    username = get_username()

    run_image_customizer(
        docker_client,
        image_customizer_container_url,
        input_image,
        config_path,
        username,
        ssh_public_key,
        output_format,
        output_image_path,
        close_list,
    )

    image_name = os.path.basename(output_image_path)
    image_name_without_ext, image_ext = os.path.splitext(image_name)
    customized_image_name = (
        image_name_without_ext
        + "_"
        + get_host_distro()
        + "_"
        + source_boot_type
        + "_azl"
        + str(input_image_azl_release)
        + "_to_"
        + target_boot_type
        + image_ext
    )
    customized_image_path = str(logs_dir) + "/" + customized_image_name
    vm_console_log_file_path = customized_image_path + ".console.log"
    logging.debug(f"- vm_console_log_file_path = {vm_console_log_file_path}")

    vm_image = output_image_path
    if output_format != "iso":
        diff_image_path = test_temp_dir.joinpath("image-diff.qcow2")

        # Create a differencing disk for the VM.
        # This will make it easier to manually debug what is in the image itself and what was set during first boot.
        local_client.run(
            [
                "qemu-img",
                "create",
                "-F",
                output_format,
                "-f",
                "qcow2",
                "-b",
                str(output_image_path),
                str(diff_image_path),
            ],
        ).check_exit_code()

        # Ensure VM can write to the disk file.
        os.chmod(diff_image_path, 0o666)

        vm_image = diff_image_path

    # Create VM.
    vm_name = test_instance_name

    vm_spec = VmSpec(vm_name, 4096, 4, vm_image, target_boot_type, secure_boot)
    domain_xml = create_libvirt_domain_xml(libvirt_conn, vm_spec)

    logging.debug(f"\n\ndomain_xml            = {domain_xml}\n\n")

    vm = LibvirtVm(vm_name, domain_xml, vm_console_log_file_path, libvirt_conn)
    close_list.append(vm)

    # Start VM.
    vm.start()

    # Connect to the VM.
    with vm.create_ssh_client(ssh_private_key_path, test_temp_dir, username) as ssh_client:
        # Run the test
        run_basic_checks(ssh_client, input_image_azl_release, test_temp_dir)


def run_basic_checks(
    ssh_client: SshClient,
    input_image_azl_release: int,
    test_temp_dir: Path,
) -> None:

    ssh_client.run("cat /proc/cmdline").check_exit_code()

    os_release_path = test_temp_dir.joinpath("os-release")
    ssh_client.get_file(Path("/etc/os-release"), os_release_path)

    with open(os_release_path, "r") as os_release_fd:
        os_release_text = os_release_fd.read()

        if input_image_azl_release == 2:
            assert "ID=mariner" in os_release_text
            assert 'VERSION_ID="2.0"' in os_release_text
        elif input_image_azl_release == 3:
            assert "ID=azurelinux" in os_release_text
            assert 'VERSION_ID="3.0"' in os_release_text
        else:
            assert False, "Unexpected image identity in /etc/os-release"


@pytest.mark.skipif(platform.machine() != "x86_64", reason="arm64 is not supported for this combination")
def test_min_change_efi_azl2_qcow_output(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    core_efi_azl2: Path,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
    logs_dir: Path,
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
) -> None:
    azl_release = 2
    config_path = TEST_CONFIGS_DIR.joinpath("nochange-config.yaml")
    output_format = "qcow2"

    run_min_change_test(
        docker_client,
        image_customizer_container_url,
        core_efi_azl2,
        azl_release,
        config_path,
        output_format,
        ssh_key,
        test_temp_dir,
        test_instance_name,
        logs_dir,
        libvirt_conn,
        close_list,
    )


def test_min_change_efi_azl3_qcow_output(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    core_efi_azl3: Path,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
    logs_dir: Path,
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
) -> None:
    azl_release = 3
    if platform.machine() == "x86_64":
        config_path = TEST_CONFIGS_DIR.joinpath("os-vm-config.yaml")
    else:
        config_path = TEST_CONFIGS_DIR.joinpath("os-vm-config-arm64.yaml")
    output_format = "qcow2"

    run_min_change_test(
        docker_client,
        image_customizer_container_url,
        core_efi_azl3,
        azl_release,
        config_path,
        output_format,
        ssh_key,
        test_temp_dir,
        test_instance_name,
        logs_dir,
        libvirt_conn,
        close_list,
    )


@pytest.mark.skipif(platform.machine() != "x86_64", reason="arm64 is not supported for this combination")
def test_min_change_legacy_azl2_qcow_output(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    core_legacy_azl2: Path,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
    logs_dir: Path,
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
) -> None:
    azl_release = 2
    config_path = TEST_CONFIGS_DIR.joinpath("nochange-config.yaml")
    output_format = "qcow2"

    run_min_change_test(
        docker_client,
        image_customizer_container_url,
        core_legacy_azl2,
        azl_release,
        config_path,
        output_format,
        ssh_key,
        test_temp_dir,
        test_instance_name,
        logs_dir,
        libvirt_conn,
        close_list,
    )


@pytest.mark.skipif(platform.machine() != "x86_64", reason="no arm64 legacy boot input images are available")
def test_min_change_legacy_azl3_qcow_output(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    core_legacy_azl3: Path,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
    logs_dir: Path,
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
) -> None:
    azl_release = 3
    config_path = TEST_CONFIGS_DIR.joinpath("os-vm-config.yaml")
    output_format = "qcow2"

    run_min_change_test(
        docker_client,
        image_customizer_container_url,
        core_legacy_azl3,
        azl_release,
        config_path,
        output_format,
        ssh_key,
        test_temp_dir,
        test_instance_name,
        logs_dir,
        libvirt_conn,
        close_list,
    )


@pytest.mark.skipif(platform.machine() != "x86_64", reason="arm64 is not supported for this combination")
def test_min_change_efi_azl2_iso_output(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    core_efi_azl2: Path,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
    logs_dir: Path,
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
) -> None:
    azl_release = 2
    config_path = TEST_CONFIGS_DIR.joinpath("iso-bootstrap-vm-azl2.yaml")
    output_format = "iso"

    run_min_change_test(
        docker_client,
        image_customizer_container_url,
        core_efi_azl2,
        azl_release,
        config_path,
        output_format,
        ssh_key,
        test_temp_dir,
        test_instance_name,
        logs_dir,
        libvirt_conn,
        close_list,
    )


def test_min_change_efi_azl3_iso_bootstrap_output(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    core_efi_azl3: Path,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
    logs_dir: Path,
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
) -> None:
    azl_release = 3
    config_path = TEST_CONFIGS_DIR.joinpath("iso-bootstrap-vm.yaml")
    output_format = "iso"

    run_min_change_test(
        docker_client,
        image_customizer_container_url,
        core_efi_azl3,
        azl_release,
        config_path,
        output_format,
        ssh_key,
        test_temp_dir,
        test_instance_name,
        logs_dir,
        libvirt_conn,
        close_list,
    )


def test_min_change_efi_azl3_iso_full_os_output(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    core_efi_azl3: Path,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
    logs_dir: Path,
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
) -> None:
    azl_release = 3
    config_path = TEST_CONFIGS_DIR.joinpath("iso-full-os-vm.yaml")
    output_format = "iso"

    run_min_change_test(
        docker_client,
        image_customizer_container_url,
        core_efi_azl3,
        azl_release,
        config_path,
        output_format,
        ssh_key,
        test_temp_dir,
        test_instance_name,
        logs_dir,
        libvirt_conn,
        close_list,
    )


@pytest.mark.skipif(platform.machine() != "x86_64", reason="arm64 is not supported for this combination")
def test_min_change_legacy_azl2_iso_output(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    core_legacy_azl2: Path,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
    logs_dir: Path,
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
) -> None:
    azl_release = 2
    config_path = TEST_CONFIGS_DIR.joinpath("iso-bootstrap-vm-azl2.yaml")
    output_format = "iso"

    run_min_change_test(
        docker_client,
        image_customizer_container_url,
        core_legacy_azl2,
        azl_release,
        config_path,
        output_format,
        ssh_key,
        test_temp_dir,
        test_instance_name,
        logs_dir,
        libvirt_conn,
        close_list,
    )


@pytest.mark.skipif(platform.machine() != "x86_64", reason="no arm64 legacy boot input images are available")
def test_min_change_legacy_azl3_iso_output(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    core_legacy_azl3: Path,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
    logs_dir: Path,
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
) -> None:
    azl_release = 3
    config_path = TEST_CONFIGS_DIR.joinpath("iso-bootstrap-vm.yaml")
    output_format = "iso"

    run_min_change_test(
        docker_client,
        image_customizer_container_url,
        core_legacy_azl3,
        azl_release,
        config_path,
        output_format,
        ssh_key,
        test_temp_dir,
        test_instance_name,
        logs_dir,
        libvirt_conn,
        close_list,
    )
