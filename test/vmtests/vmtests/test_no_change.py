# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import os
from getpass import getuser
import logging
from pathlib import Path
import shlex
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
    input_image: Path,
    output_format: str,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
) -> None:
    (ssh_public_key, ssh_private_key_path) = ssh_key

    if output_format != "iso":
        config_path = TEST_CONFIGS_DIR.joinpath("nochange-config.yaml")
    else:
        config_path = TEST_CONFIGS_DIR.joinpath("nochange-iso-config.yaml")

    output_image_path = test_temp_dir.joinpath("image." + output_format)

    boot_type = "efi"
    if Path(input_image).suffix.lower() == ".vhd" and output_format != "iso":
        boot_type = "legacy"

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

    logging.debug(f"---- debug ---- [1] -- output_image_path={output_image_path}")

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

    logging.debug(f"---- debug ---- [2] -- creating domain xml - vm_image={vm_image}")

    # Create VM.
    vm_name = test_instance_name

    domain_xml = create_libvirt_domain_xml(VmSpec(vm_name, 4096, 4, vm_image), get_host_os(), boot_type)

    logging.debug(f"---- debug ---- [3] -- creating domain - domain_xml={domain_xml}")

    vm = LibvirtVm(vm_name, domain_xml, libvirt_conn)
    close_list.append(vm)

    logging.debug("---- debug ---- [4] -- starting vm")

    # Start VM.
    vm.start()

    logging.debug("---- debug ---- [5] -- getting ip address...")

    # Wait for VM to boot by waiting for it to request an IP address from the DHCP server.
    vm_ip_address = vm.get_vm_ip_address(timeout=30)
    logging.debug("---- debug ---- [6] -- waiting for the vm to be ready to connect to...")
    # iso booting takes longer due to the copying of artifacts to memory
    time.sleep(30)

    logging.debug("---- debug ---- [7] -- connecting to vm...")
    # Connect to VM using SSH.
    ssh_known_hosts_path = test_temp_dir.joinpath("known_hosts")
    open(ssh_known_hosts_path, "w").close()

    with SshClient(vm_ip_address, key_path=ssh_private_key_path, known_hosts_path=ssh_known_hosts_path) as vm_ssh:
        logging.debug("---- debug ---- [8] -- running tests...")
        vm_ssh.run("cat /proc/cmdline").check_exit_code()

        os_release_path = test_temp_dir.joinpath("os-release")
        vm_ssh.get_file(Path("/etc/os-release"), os_release_path)

        with open(os_release_path, "r") as os_release_fd:
            os_release_text = os_release_fd.read()

            assert ("ID=azurelinux" in os_release_text) or ("ID=mariner" in os_release_text)
            assert ('VERSION_ID="3.0"' in os_release_text) or ('VERSION_ID="2.0"' in os_release_text)

        logging.debug("---- debug ---- [9] -- done running tests.")

def get_host_os () -> str:
    with open("/etc/os-release", "r") as f:
        for line in f:
            key, _, value = line.partition("=")
            if key == "NAME":
                os_name = shlex.split(value)[0]  # Safely handle quoted values
                print(f"OS Name: {os_name}")
                break

    return os_name
