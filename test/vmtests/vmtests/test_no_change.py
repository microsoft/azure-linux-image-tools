# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import os
from getpass import getuser
import logging
from pathlib import Path
import time
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


def test_no_change(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    core_efi_azl2: Path,
    core_efi_azl3: Path,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
) -> None:
    (ssh_public_key, ssh_private_key_path) = ssh_key

    config_path = TEST_CONFIGS_DIR.joinpath("nochange-config.yaml")
    # output_image_format="qcow2"
    output_image_format="iso"
    output_image_path = test_temp_dir.joinpath("image."+ output_image_format)
    diff_image_path = test_temp_dir.joinpath("image-diff.qcow2")

    print(f"---- debug ---- core_efi_azl2:({core_efi_azl2.absolute()})")
    print(f"---- debug ---- core_efi_azl2:({core_efi_azl2.name})")
    print(f"---- debug ---- core_efi_azl3:({core_efi_azl3.absolute()})")
    print(f"---- debug ---- core_efi_azl3:({core_efi_azl3.name})")
    core_efi_azlx = core_efi_azl2
    if core_efi_azl2.name == "":
        core_efi_azlx = core_efi_azl3
    print(f"---- debug ---- core_efi_azlx:({core_efi_azlx.absolute()})")

    username = getuser()

    run_image_customizer(
        docker_client,
        image_customizer_container_url,
        core_efi_azlx,
        config_path,
        username,
        ssh_public_key,
        "iso",
        output_image_path,
        close_list,
    )

    vm_image = output_image_path

    if output_image_format == "qcow2":
        # Create a differencing disk for the VM.
        # This will make it easier to manually debug what is in the image itself and what was set during first boot.
        local_client.run(
            ["qemu-img", "create", "-F", "qcow2", "-f", "qcow2", "-b", str(output_image_path), str(diff_image_path)],
        ).check_exit_code()

        # Ensure VM can write to the disk file.
        os.chmod(diff_image_path, 0o666)

        vm_image = diff_image_path

    logging.debug("---- debug ---- [1] creating the VM")

    # Create VM.
    vm_name = test_instance_name
    domain_xml = create_libvirt_domain_xml(VmSpec(vm_name, 4096, 4, vm_image))

    logging.debug(f"---- debug ---- [2] {domain_xml}")

    vm = LibvirtVm(vm_name, domain_xml, libvirt_conn)
    close_list.append(vm)

    logging.debug("---- debug ---- [3] starting the VM")

    # Start VM.
    vm.start()

    logging.debug("---- debug ---- [4] getting its ip address")

    # Wait for VM to boot by waiting for it to request an IP address from the DHCP server.
    vm_ip_address = vm.get_vm_ip_address(timeout=90)

    logging.debug(f"---- debug ---- [5] got the ip address {vm_ip_address} - now pausing for 30 seconds")

    time.sleep(30)

    logging.debug(f"---- debug ---- [5] got the ip address {vm_ip_address} - now connecting using ssh")

    # Connect to VM using SSH.
    ssh_known_hosts_path = test_temp_dir.joinpath("known_hosts")
    open(ssh_known_hosts_path, "w").close()

    with SshClient(vm_ip_address, key_path=ssh_private_key_path, known_hosts_path=ssh_known_hosts_path) as vm_ssh:

        logging.debug("---- debug ---- [6] connected using ssh - running commands")

        vm_ssh.run("cat /proc/cmdline").check_exit_code()

        os_release_path = test_temp_dir.joinpath("os-release")
        vm_ssh.get_file(Path("/etc/os-release"), os_release_path)

        with open(os_release_path, "r") as os_release_fd:
            os_release_text = os_release_fd.read()

            assert "ID=azurelinux" in os_release_text
            assert 'VERSION_ID="3.0"' in os_release_text

    logging.debug("---- debug ---- [7] test completed")