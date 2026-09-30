# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import hashlib
import logging
import platform
import random
import shutil
import string
from pathlib import Path
from typing import List, Tuple

import libvirt  # type: ignore
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
    boot_count: int = 1,
) -> None:

    if not 1 <= boot_count <= 20:
        raise ValueError("boot_count must be between 1 and 20")

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

    # Diagnostic branch: build once so every fresh VM boots exactly the same bytes.
    evidence_dir = logs_dir / test_instance_name
    evidence_dir.mkdir(parents=True, exist_ok=True)
    with pxe_tar_path.open("rb") as image_file:
        image_sha256 = hashlib.file_digest(image_file, "sha256").hexdigest()
    (evidence_dir / "image.sha256").write_text(f"{image_sha256}  pxe-artifacts.tar.gz\n")
    shutil.copy2(modified_config_path, evidence_dir / "config.yaml")

    suffix = "".join(random.choice(string.ascii_lowercase) for _ in range(5))
    network_name = test_instance_name + "-pxe"
    bridge_name = "pxebr" + suffix

    pxe_env = PxeEnvironment(
        libvirt_conn,
        network_name,
        bridge_name,
        pxe_tar_path,
        boot_loader_file,
        http_log_file_path,
    )
    close_list.append(pxe_env)

    for boot_index in range(1, boot_count + 1):
        vm_name = f"{test_instance_name}-boot{boot_index:02d}"
        boot_dir = evidence_dir / f"boot{boot_index:02d}"
        boot_dir.mkdir()
        capture_host_resources(boot_dir / "host-before.txt")
        logging.info("PXE diagnostic boot %d/%d starting, image_sha256=%s", boot_index, boot_count, image_sha256)

        # Keep resources, boot configuration, and SSH checks identical to the original test.
        vm_spec = VmSpec(
            vm_name,
            PXE_VM_MEMORY_MIB,
            PXE_VM_CORE_COUNT,
            None,
            "efi",
            secure_boot=False,
            pxe_boot=True,
            network_name=pxe_env.network_name,
        )
        domain_xml = create_libvirt_domain_xml(libvirt_conn, vm_spec)
        (boot_dir / "domain.requested.xml").write_text(domain_xml)
        vm = LibvirtVm(vm_name, domain_xml, str(boot_dir / "console.log"), libvirt_conn)
        close_list.append(vm)

        try:
            vm.start()
            (boot_dir / "domain.active.xml").write_text(vm.domain.XMLDesc(0))
            with vm.create_ssh_client(
                ssh_private_key_path,
                test_temp_dir,
                username,
                ip_wait_time_extra=PXE_BOOT_IP_WAIT_TIME_EXTRA_SECONDS,
            ) as ssh_client:
                run_basic_checks(ssh_client, input_image_azl_release, test_temp_dir)
        except Exception:
            logging.exception("PXE diagnostic boot %d/%d failed", boot_index, boot_count)
            try:
                shutil.copy2(pxe_tar_path, evidence_dir / "pxe-artifacts.tar.gz")
            except OSError:
                logging.exception("Could not preserve the failing PXE image")
            raise
        finally:
            capture_host_resources(boot_dir / "host-after.txt")
            qemu_log = Path("/var/log/libvirt/qemu") / f"{vm_name}.log"
            try:
                if qemu_log.is_file():
                    shutil.copy2(qemu_log, boot_dir / "qemu.log")
            except OSError:
                logging.exception("Could not preserve the QEMU log")

        logging.info("PXE diagnostic boot %d/%d passed", boot_index, boot_count)
        if boot_index < boot_count:
            # Stop and undefine successful guests before starting the next one.
            # On failure, the existing fixture still owns cleanup and no later boot is attempted.
            vm.close()
            close_list.remove(vm)


def capture_host_resources(output_path: Path) -> None:
    try:
        snapshot = ""
        for name in ("uptime", "loadavg", "meminfo"):
            snapshot += f"/proc/{name}\n{Path('/proc', name).read_text()}\n"
        output_path.write_text(snapshot)
    except OSError:
        logging.exception("Could not record host resources")


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
        boot_count=20,
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
