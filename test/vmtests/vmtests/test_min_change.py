# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import os
from getpass import getuser
import logging
from pathlib import Path
import shlex
from typing import List, Tuple

import libvirt  # type: ignore
from docker import DockerClient

from .conftest import TEST_CONFIGS_DIR
from .utils import local_client
from .utils.closeable import Closeable
from .utils.imagecustomizer import run_image_customizer
from .utils.libvirt_utils import VmSpec, create_libvirt_domain_xml
from .utils.libvirt_vm import LibvirtVm
from .utils.ssh_client import SshClient


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
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
) -> None:

    (ssh_public_key, ssh_private_key_path) = ssh_key

    secure_boot = False

    boot_type = "efi"
    if Path(input_image).suffix.lower() == ".vhd" and output_format != "iso":
        boot_type = "legacy"

    output_image_path = test_temp_dir.joinpath("image." + output_format)

    logging.debug(f"Test parameters:")
    logging.debug(f"- input_image             = {input_image}")
    logging.debug(f"- input_image_azl_release = {input_image_azl_release}")
    logging.debug(f"- config_path             = {config_path}")
    logging.debug(f"- output_format           = {output_format}")
    logging.debug(f"- boot_type               = {boot_type}")

    username = getuser()

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

    vm_image = output_image_path
    if output_format != "iso":
        diff_image_path = test_temp_dir.joinpath("image-diff.qcow2")

        # Create a differencing disk for the VM.
        # This will make it easier to manually debug what is in the image itself and what was set during first boot.
        local_client.run(
            ["qemu-img", "create", "-F", "qcow2", "-f", "qcow2", "-b", str(output_image_path), str(diff_image_path)],
        ).check_exit_code()

        # Ensure VM can write to the disk file.
        os.chmod(diff_image_path, 0o666)

        vm_image = diff_image_path

    # Create VM.
    vm_name = test_instance_name

    domain_xml = create_libvirt_domain_xml(libvirt_conn, VmSpec(vm_name, 4096, 4, vm_image, boot_type, secure_boot))

    logging.debug(f"\n\ndomain_xml            = {domain_xml}\n\n")

    vm = LibvirtVm(vm_name, domain_xml, libvirt_conn)
    close_list.append(vm)

    # Start VM.
    vm.start()

    # Wait for VM to boot by waiting for it to request an IP address from the DHCP server.
    vm_ip_address = vm.get_vm_ip_address(timeout=30)

    # Connect to VM using SSH.
    ssh_known_hosts_path = test_temp_dir.joinpath("known_hosts")
    open(ssh_known_hosts_path, "w").close()

    with SshClient(vm_ip_address, key_path=ssh_private_key_path, known_hosts_path=ssh_known_hosts_path) as vm_ssh:
        vm_ssh.run("cat /proc/cmdline").check_exit_code()

        os_release_path = test_temp_dir.joinpath("os-release")
        vm_ssh.get_file(Path("/etc/os-release"), os_release_path)

        with open(os_release_path, "r") as os_release_fd:
            os_release_text = os_release_fd.read()

            if input_image_azl_release == 2:
                assert ("ID=mariner" in os_release_text)
                assert ('VERSION_ID="2.0"' in os_release_text)
            elif input_image_azl_release == 3:
                assert ("ID=azurelinux" in os_release_text)
                assert ('VERSION_ID="3.0"' in os_release_text)
            else:
                assert False, "Unexpected image identity in /etc/os-release"


def test_min_change_efi_azl2_qcow_output(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    core_efi_azl2: Path,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
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
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
) -> None:
    azl_release = 3
    config_path = TEST_CONFIGS_DIR.joinpath("os-vm-config.yaml")
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
        libvirt_conn,
        close_list,
    )


def test_min_change_legacy_azl2_qcow_output(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    core_legacy_azl2: Path,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
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
        libvirt_conn,
        close_list,
    )


def test_min_change_legacy_azl3_qcow_output(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    core_legacy_azl3: Path,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
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
        libvirt_conn,
        close_list,
    )


def test_min_change_efi_azl2_iso_output(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    core_efi_azl2: Path,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
) -> None:
    azl_release = 2
    config_path = TEST_CONFIGS_DIR.joinpath("iso-os-vm-config.yaml")
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
        libvirt_conn,
        close_list,
    )

def test_min_change_efi_azl3_iso_output(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    core_efi_azl3: Path,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
) -> None:
    azl_release = 3
    config_path = TEST_CONFIGS_DIR.joinpath("iso-os-vm-config.yaml")
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
        libvirt_conn,
        close_list,
    )


def test_min_change_legacy_azl2_iso_output(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    core_legacy_azl2: Path,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
) -> None:
    azl_release = 2
    config_path = TEST_CONFIGS_DIR.joinpath("iso-os-vm-config.yaml")
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
        libvirt_conn,
        close_list,
    )


def test_min_change_legacy_azl3_iso_output(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    core_legacy_azl3: Path,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
) -> None:
    azl_release = 3
    config_path = TEST_CONFIGS_DIR.joinpath("iso-os-vm-config.yaml")
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
        libvirt_conn,
        close_list,
    )
