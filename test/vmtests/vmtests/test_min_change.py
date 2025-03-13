# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import os
from getpass import getuser
import logging
from pathlib import Path
import shlex
import time
from typing import List, Tuple
import shutil

import libvirt  # type: ignore
from docker import DockerClient

from .conftest import TEST_CONFIGS_DIR
from .utils import local_client
from .utils.closeable import Closeable
from .utils.imagecustomizer import run_image_customizer
from .utils.libvirt_utils import VmSpec, create_libvirt_domain_xml
from .utils.libvirt_vm import LibvirtVm
from .utils.ssh_client import SshClient


def get_host_distro() -> str:
    file_path = "/etc/os-release"
    name_value = ""
    with open(file_path, "r") as file:
        for line in file:
            if line.startswith("ID="):
                name_value = line.strip().split("=", 1)[1]  # Get the value part
                break
    return name_value

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
    artifacts_folder: Path,
    rpms_folder: Path,
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
    logging.debug(f"- artifacts_folder        = {artifacts_folder}")
    logging.debug(f"- rpms_folder             = {rpms_folder}")

    username = getuser()

    run_image_customizer(
        docker_client,
        image_customizer_container_url,
        input_image,
        config_path,
        rpms_folder,
        username,
        ssh_public_key,
        output_format,
        output_image_path,
        close_list,
    )

    image_name = os.path.basename(output_image_path)
    image_name_without_ext, image_ext = os.path.splitext(image_name)
    new_image_name = str(artifacts_folder) + "/" + image_name_without_ext + "_" + boot_type + "_" + get_host_distro() + image_ext

    logging.debug(f"-- copying {output_image_path} to {new_image_name}")
    shutil.copy2(output_image_path, new_image_name)

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

    if get_host_distro() == "ubuntu":

        local_client.run(
            ["virt-install",
            "--name", "prism_arm64_iso",
            "--memory", "4096",
            "--vcpus", "4",
            "--os-type", "Linux",
            "--os-variant", "generic",
            "--console", "pty,target_type=serial",
            "--cdrom", vm_image,
            "--disk", "none"
            "--virt-type", "qemu",
            "--arch", "aarch64",
            "--noautoconsole"
            ])

        time.sleep(10)

        local_client.run(
            ["virsh",
            "--connect", "qemu:///system",
            "list"])

        local_client.run(
            ["virsh",
            "--connect", "qemu:///system",
            "dumpxml", "prism_arm64_iso"])

    # "--connect", "qemu:///system",
    # "--name", "PXE-client",
    # "--ram", "4096",
    # "--vcpus=1",
    # "--osinfo", "generic",
    # "--disk", "/var/lib/libvirt/images/PXE-client-aarch64.qcow2,size=40",
    # "--os-variant", "generic",
    # "--noautoconsole",
    # "--graphics", "none",
    # "--serial=pty",
    # "--network", "bridge=virbr0",
    # "--check", "path_in_use=off",
    # "--machine", "virt",
    # "--arch", "aarch64",
    # "--cpu", "cortex-a57",
    # "--features", "smm.state=off",
    # "--boot", "uefi,loader=/usr/share/AAVMF/AAVMF_CODE.ms.fd,loader_secure=no"])

    logging.debug(f"\n\ncreating domain xml\n\n")
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


# def test_min_change_efi_azl2_qcow_output(
#     docker_client: DockerClient,
#     image_customizer_container_url: str,
#     core_efi_azl2: Path,
#     ssh_key: Tuple[str, Path],
#     test_temp_dir: Path,
#     test_instance_name: str,
#     libvirt_conn: libvirt.virConnect,
#     close_list: List[Closeable],
# ) -> None:
#     azl_release = 2
#     config_path = TEST_CONFIGS_DIR.joinpath("nochange-config.yaml")
#     output_format = "qcow2"

#     run_min_change_test(
#         docker_client,
#         image_customizer_container_url,
#         core_efi_azl2,
#         azl_release,
#         config_path,
#         output_format,
#         ssh_key,
#         test_temp_dir,
#         test_instance_name,
#         libvirt_conn,
#         close_list,
#     )


# def test_min_change_efi_azl3_qcow_output(
#     docker_client: DockerClient,
#     image_customizer_container_url: str,
#     core_efi_azl3: Path,
#     ssh_key: Tuple[str, Path],
#     test_temp_dir: Path,
#     test_instance_name: str,
#     artifacts_folder: Path,
#     rpms_folder: Path,
#     libvirt_conn: libvirt.virConnect,
#     close_list: List[Closeable],
# ) -> None:
#     azl_release = 3
#     config_path = TEST_CONFIGS_DIR.joinpath("os-vm-config.yaml")
#     output_format = "qcow2"

#     run_min_change_test(
#         docker_client,
#         image_customizer_container_url,
#         core_efi_azl3,
#         azl_release,
#         config_path,
#         output_format,
#         ssh_key,
#         test_temp_dir,
#         test_instance_name,
#         artifacts_folder,
#         rpms_folder,
#         libvirt_conn,
#         close_list,
#     )


# def test_min_change_legacy_azl2_qcow_output(
#     docker_client: DockerClient,
#     image_customizer_container_url: str,
#     core_legacy_azl2: Path,
#     ssh_key: Tuple[str, Path],
#     test_temp_dir: Path,
#     test_instance_name: str,
#     libvirt_conn: libvirt.virConnect,
#     close_list: List[Closeable],
# ) -> None:
#     azl_release = 2
#     config_path = TEST_CONFIGS_DIR.joinpath("nochange-config.yaml")
#     output_format = "qcow2"

#     run_min_change_test(
#         docker_client,
#         image_customizer_container_url,
#         core_legacy_azl2,
#         azl_release,
#         config_path,
#         output_format,
#         ssh_key,
#         test_temp_dir,
#         test_instance_name,
#         libvirt_conn,
#         close_list,
#     )


# def test_min_change_legacy_azl3_qcow_output(
#     docker_client: DockerClient,
#     image_customizer_container_url: str,
#     core_legacy_azl3: Path,
#     ssh_key: Tuple[str, Path],
#     test_temp_dir: Path,
#     test_instance_name: str,
#     libvirt_conn: libvirt.virConnect,
#     close_list: List[Closeable],
# ) -> None:
#     azl_release = 3
#     config_path = TEST_CONFIGS_DIR.joinpath("os-vm-config.yaml")
#     output_format = "qcow2"

#     run_min_change_test(
#         docker_client,
#         image_customizer_container_url,
#         core_legacy_azl3,
#         azl_release,
#         config_path,
#         output_format,
#         ssh_key,
#         test_temp_dir,
#         test_instance_name,
#         libvirt_conn,
#         close_list,
#     )


# def test_min_change_efi_azl2_iso_output(
#     docker_client: DockerClient,
#     image_customizer_container_url: str,
#     core_efi_azl2: Path,
#     ssh_key: Tuple[str, Path],
#     test_temp_dir: Path,
#     test_instance_name: str,
#     libvirt_conn: libvirt.virConnect,
#     close_list: List[Closeable],
# ) -> None:
#     azl_release = 2
#     config_path = TEST_CONFIGS_DIR.joinpath("iso-os-vm-config.yaml")
#     output_format = "iso"

#     run_min_change_test(
#         docker_client,
#         image_customizer_container_url,
#         core_efi_azl2,
#         azl_release,
#         config_path,
#         output_format,
#         ssh_key,
#         test_temp_dir,
#         test_instance_name,
#         libvirt_conn,
#         close_list,
#     )

# uncomment this one for testing azl3 iso

def test_min_change_efi_azl3_iso_output(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    core_efi_azl3: Path,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
    artifacts_folder: Path,
    rpms_folder: Path,
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
        artifacts_folder,
        rpms_folder,
        libvirt_conn,
        close_list,
    )


# def test_min_change_legacy_azl2_iso_output(
#     docker_client: DockerClient,
#     image_customizer_container_url: str,
#     core_legacy_azl2: Path,
#     ssh_key: Tuple[str, Path],
#     test_temp_dir: Path,
#     test_instance_name: str,
#     libvirt_conn: libvirt.virConnect,
#     close_list: List[Closeable],
# ) -> None:
#     azl_release = 2
#     config_path = TEST_CONFIGS_DIR.joinpath("iso-os-vm-config.yaml")
#     output_format = "iso"

#     run_min_change_test(
#         docker_client,
#         image_customizer_container_url,
#         core_legacy_azl2,
#         azl_release,
#         config_path,
#         output_format,
#         ssh_key,
#         test_temp_dir,
#         test_instance_name,
#         libvirt_conn,
#         close_list,
#     )


# def test_min_change_legacy_azl3_iso_output(
#     docker_client: DockerClient,
#     image_customizer_container_url: str,
#     core_legacy_azl3: Path,
#     ssh_key: Tuple[str, Path],
#     test_temp_dir: Path,
#     test_instance_name: str,
#     libvirt_conn: libvirt.virConnect,
#     close_list: List[Closeable],
# ) -> None:
#     azl_release = 3
#     config_path = TEST_CONFIGS_DIR.joinpath("iso-os-vm-config.yaml")
#     output_format = "iso"

#     run_min_change_test(
#         docker_client,
#         image_customizer_container_url,
#         core_legacy_azl3,
#         azl_release,
#         config_path,
#         output_format,
#         ssh_key,
#         test_temp_dir,
#         test_instance_name,
#         libvirt_conn,
#         close_list,
#     )
