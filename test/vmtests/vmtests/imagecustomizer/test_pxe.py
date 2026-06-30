# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import logging
import platform
import random
import string
import tarfile
from pathlib import Path
from typing import List, Tuple

import libvirt  # type: ignore
import pytest
from docker import DockerClient

from ..conftest import TEST_CONFIGS_DIR
from ..utils.closeable import Closeable
from ..utils.host_utils import get_host_distro
from ..utils.imagecustomizer import (
    add_preview_features_to_config,
    add_pxe_bootstrap_base_url_to_config,
    add_ssh_to_config,
    run_image_customizer,
)
from ..utils.libvirt_utils import VmSpec, create_libvirt_domain_xml
from ..utils.libvirt_vm import LibvirtVm
from ..utils.pxe_server import PXE_HTTP_PORT, PXE_NETWORK_GATEWAY_IP, PxeEnvironment
from ..utils.user_utils import get_username
from .test_min_change import run_basic_checks

# The full-OS image is downloaded into RAM during PXE bootstrap, so the VM needs more memory than a disk/ISO boot.
PXE_VM_MEMORY_MIB = 8192
PXE_VM_CORE_COUNT = 4

# PXE boot adds a firmware netboot phase plus an over-the-network bootstrap-image download before the OS requests its
# DHCP lease, so it needs additional time to boot.
PXE_BOOT_IP_WAIT_TIME_EXTRA_SECONDS = 600


def run_pxe_test(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    input_image: Path,
    input_image_azl_release: int,
    initramfs_type: str,
    config_path: Path,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
    logs_dir: Path,
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
) -> None:

    ssh_public_key, ssh_private_key_path = ssh_key

    if platform.machine() == "x86_64":
        boot_loader_file = "bootx64.efi"
    else:
        boot_loader_file = "bootaa64.efi"

    username = get_username()

    modified_config_path = add_ssh_to_config(config_path, username, ssh_public_key, close_list)

    if initramfs_type == "bootstrap":
        bootstrap_base_url = f"http://{PXE_NETWORK_GATEWAY_IP}:{PXE_HTTP_PORT}"
        modified_config_path = add_pxe_bootstrap_base_url_to_config(
            modified_config_path, bootstrap_base_url, close_list
        )

    pxe_tar_path = test_temp_dir.joinpath("pxe-artifacts.tar.gz")
    run_image_customizer(
        docker_client,
        image_customizer_container_url,
        "customize",
        modified_config_path,
        "pxe-tar",
        pxe_tar_path,
        image_file=input_image,
    )

    pxe_artifacts_dir = test_temp_dir.joinpath("pxe-artifacts")
    pxe_artifacts_dir.mkdir()
    with tarfile.open(pxe_tar_path, "r:gz") as tar:
        tar.extractall(pxe_artifacts_dir)

    customized_name = (
        "pxe_"
        + initramfs_type.replace("-", "_")
        + "_"
        + get_host_distro()
        + "_efi_azl"
        + str(input_image_azl_release)
        + "_to_efi"
    )
    customized_log_path = str(logs_dir) + "/" + customized_name
    http_log_file_path = Path(customized_log_path + ".http.log")
    vm_console_log_file_path = customized_log_path + ".console.log"

    suffix = "".join(random.choice(string.ascii_lowercase) for _ in range(5))
    network_name = test_instance_name + "-pxe"
    bridge_name = "pxebr" + suffix

    pxe_env = PxeEnvironment(
        libvirt_conn,
        network_name,
        bridge_name,
        pxe_artifacts_dir,
        boot_loader_file,
        http_log_file_path,
    )
    close_list.append(pxe_env)

    # Create the VM: no disk, boots from the PXE network.
    vm_spec = VmSpec(
        test_instance_name,
        PXE_VM_MEMORY_MIB,
        PXE_VM_CORE_COUNT,
        None,
        "efi",
        secure_boot=False,
        pxe_boot=True,
        network_name=pxe_env.network_name,
    )
    domain_xml = create_libvirt_domain_xml(libvirt_conn, vm_spec)
    logging.debug(f"\n\ndomain_xml = {domain_xml}\n\n")

    vm = LibvirtVm(test_instance_name, domain_xml, vm_console_log_file_path, libvirt_conn)
    close_list.append(vm)

    # Start the VM.
    vm.start()

    # Connect to the VM and run the basic boot validation.
    with vm.create_ssh_client(
        ssh_private_key_path,
        test_temp_dir,
        username,
        ip_wait_time_extra=PXE_BOOT_IP_WAIT_TIME_EXTRA_SECONDS,
    ) as ssh_client:
        run_basic_checks(ssh_client, input_image_azl_release, test_temp_dir)


@pytest.mark.skipif(
    get_host_distro() == "azurelinux",
    reason="PXE requires a network-enabled host UEFI firmware (EDK II NetworkPkg), which only Ubuntu hosts provide",
)
def test_pxe_bootstrap_efi_azl3(
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
    config_path = TEST_CONFIGS_DIR.joinpath("pxe-bootstrap-vm-azl3.yaml")

    run_pxe_test(
        docker_client,
        image_customizer_container_url,
        core_efi_azl3,
        azl_release,
        "bootstrap",
        config_path,
        ssh_key,
        test_temp_dir,
        test_instance_name,
        logs_dir,
        libvirt_conn,
        close_list,
    )


@pytest.mark.skipif(
    get_host_distro() == "azurelinux",
    reason="PXE requires a network-enabled host UEFI firmware (EDK II NetworkPkg), which only Ubuntu hosts provide",
)
def test_pxe_bootstrap_efi_azl4(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    core_efi_azl4: Path,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
    logs_dir: Path,
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
) -> None:
    azl_release = 4
    config_path = TEST_CONFIGS_DIR.joinpath("pxe-bootstrap-vm-azl4.yaml")
    config_path = add_preview_features_to_config(config_path, "preview-distro-version", close_list)

    run_pxe_test(
        docker_client,
        image_customizer_container_url,
        core_efi_azl4,
        azl_release,
        "bootstrap",
        config_path,
        ssh_key,
        test_temp_dir,
        test_instance_name,
        logs_dir,
        libvirt_conn,
        close_list,
    )


@pytest.mark.skipif(
    get_host_distro() == "azurelinux",
    reason="PXE requires a network-enabled host UEFI firmware (EDK II NetworkPkg), which only Ubuntu hosts provide",
)
def test_pxe_full_os_efi_azl3(
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
    config_path = TEST_CONFIGS_DIR.joinpath("pxe-full-os-vm-azl3.yaml")

    run_pxe_test(
        docker_client,
        image_customizer_container_url,
        core_efi_azl3,
        azl_release,
        "full-os",
        config_path,
        ssh_key,
        test_temp_dir,
        test_instance_name,
        logs_dir,
        libvirt_conn,
        close_list,
    )


@pytest.mark.skipif(
    get_host_distro() == "azurelinux",
    reason="PXE requires a network-enabled host UEFI firmware (EDK II NetworkPkg), which only Ubuntu hosts provide",
)
def test_pxe_full_os_efi_azl4(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    core_efi_azl4: Path,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
    logs_dir: Path,
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
) -> None:
    azl_release = 4
    config_path = TEST_CONFIGS_DIR.joinpath("pxe-full-os-vm-azl4.yaml")
    config_path = add_preview_features_to_config(config_path, "preview-distro-version", close_list)

    run_pxe_test(
        docker_client,
        image_customizer_container_url,
        core_efi_azl4,
        azl_release,
        "full-os",
        config_path,
        ssh_key,
        test_temp_dir,
        test_instance_name,
        logs_dir,
        libvirt_conn,
        close_list,
    )
