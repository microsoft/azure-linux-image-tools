# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import os
from pathlib import Path
from typing import List

import libvirt  # type: ignore
from docker import DockerClient

from .conftest import TEST_CONFIGS_DIR
from .utils import local_client
from .utils.closeable import Closeable
from .utils.imagecustomizer import run_image_customizer
from .utils.libvirt_utils import VmSpec, create_libvirt_domain_xml
from .utils.libvirt_vm import LibvirtVm


def test_no_change(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    core_efi_azl2: Path,
    test_temp_dir: Path,
    test_instance_name: str,
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
) -> None:
    config_path = TEST_CONFIGS_DIR.joinpath("nochange-config.yaml")
    output_image_path = test_temp_dir.joinpath("image.qcow2")
    diff_image_path = test_temp_dir.joinpath("image-diff.qcow2")

    run_image_customizer(
        docker_client,
        image_customizer_container_url,
        core_efi_azl2,
        config_path,
        "qcow2",
        output_image_path,
    )

    # Create a differencing disk for the VM.
    # This will make it easier to manually debug what is in the image itself and what was set during first boot.
    local_client.run(
        ["qemu-img", "create", "-F", "qcow2", "-f", "qcow2", "-b", str(output_image_path), str(diff_image_path)],
    ).check_exit_code()

    # Ensure VM can write to the disk file.
    os.chmod(diff_image_path, 0o666)

    # Create VM.
    vm_name = test_instance_name
    domain_xml = create_libvirt_domain_xml(VmSpec(vm_name, 4096, 4, diff_image_path))

    vm = LibvirtVm(vm_name, domain_xml, libvirt_conn)
    close_list.append(vm)

    # Start VM.
    vm.start()

    # Wait for VM to boot by waiting for it to request an IP address from the DHCP server.
    vm.get_vm_ip_address(timeout=15)
