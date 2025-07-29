# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import logging
import os
import platform
import tempfile
from pathlib import Path
from typing import List, Tuple

import libvirt  # type: ignore
import pytest
import yaml

from .utils import local_client
from .utils.closeable import Closeable
from .utils.file_utils import RemoveFileOnClose
from .utils.host_utils import get_host_distro
from .utils.imagecreator import run_image_creator, run_image_customizer_binary
from .utils.libvirt_utils import VmSpec, create_libvirt_domain_xml
from .utils.libvirt_vm import LibvirtVm
from .utils.ssh_client import SshClient
from .utils.user_utils import get_username

# Path to imagecreator test configs
IMAGECREATOR_TEST_CONFIGS_DIR = Path(__file__).parent.parent.parent.parent.joinpath(
    "toolkit/tools/pkg/imagecreatorlib/testdata"
)


def create_repo_file(test_temp_dir: Path) -> Path:
    repo_content = """[azurelinux-official-base]
name=Azure Linux Official Base $releasever $basearch
baseurl=https://packages.microsoft.com/azurelinux/$releasever/prod/base/$basearch
gpgcheck=0
repo_gpgcheck=0
enabled=1
skip_if_unavailable=True
sslverify=1

[azurelinux-official-base-debuginfo]
name=Azure Linux Official Base $releasever $basearch Debuginfo
baseurl=https://packages.microsoft.com/azurelinux/$releasever/prod/base/debuginfo/$basearch
gpgcheck=0
repo_gpgcheck=0
enabled=0
skip_if_unavailable=True
sslverify=1

[azurelinux-official-base-source]
name=Azure Linux Official Base $releasever Source
baseurl=https://packages.microsoft.com/azurelinux/$releasever/prod/base/srpms
gpgcheck=0
repo_gpgcheck=0
enabled=0
skip_if_unavailable=True
sslverify=1
"""

    repo_file_path = test_temp_dir.joinpath("azurelinux.repo")
    repo_file_path.write_text(repo_content)
    return repo_file_path


def run_image_creator_test(
    image_creator_binary_path: Path,
    rpm_sources: List[Path],
    tools_tar: Path,
    config_path: Path,
    output_format: str,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
    logs_dir: Path,
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
    image_customizer_binary_path: Path,
) -> None:

    (ssh_public_key, ssh_private_key_path) = ssh_key

    secure_boot = False
    target_boot_type = "efi"

    # Step 1: Create initial image with imagecreator
    initial_output_image_path = test_temp_dir.joinpath("initial-image." + output_format)
    build_dir = test_temp_dir.joinpath("build")
    build_dir.mkdir(exist_ok=True)

    logging.info(f"Step 1: Creating initial image with imagecreator...")
    logging.debug(f"Test parameters:")
    logging.debug(f"- image_creator_binary = {image_creator_binary_path}")
    logging.debug(f"- rpm_sources          = {rpm_sources}")
    logging.debug(f"- tools_tar            = {tools_tar}")
    logging.debug(f"- config_path          = {config_path}")
    logging.debug(f"- output_format        = {output_format}")
    logging.debug(f"- target_boot_type     = {target_boot_type}")
    logging.debug(f"- build_dir            = {build_dir}")
    logging.debug(f"- logs_dir             = {logs_dir}")

    username = get_username()

    run_image_creator(
        image_creator_binary_path,
        rpm_sources,
        tools_tar,
        config_path,
        output_format,
        initial_output_image_path,
        build_dir,
    )

    # Step 1.5: Run imagecustomizer to add SSH configuration
    logging.info(f"Step 1.5: Running imagecustomizer to add SSH configuration...")

    customized_output_image_path = test_temp_dir.joinpath("customized-image." + output_format)
    customizer_build_dir = test_temp_dir.joinpath("customizer-build")
    customizer_build_dir.mkdir(exist_ok=True)

    # Create a minimal imagecustomizer config for SSH
    customizer_config_content = {
        "os": {
            "additionalFiles": [
                {
                    "content": (
                        "# SPDX-License-Identifier: MIT-0\n"
                        "#\n"
                        "# This example config file is installed as part of systemd.\n"
                        "# It may be freely copied and edited (following the MIT No Attribution license).\n"
                        "#\n"
                        "# To use the file, one of the following methods may be used:\n"
                        "# 1. add a symlink from /etc/systemd/network to the current location of this file,\n"
                        "# 2. copy the file into /etc/systemd/network or one of the other paths checked\n"
                        "#    by systemd-networkd and edit it there.\n"
                        "# This file should not be edited in place, because it'll be overwritten on upgrades.\n"
                        "\n"
                        "# Enable DHCPv4 and DHCPv6 on all physical ethernet links\n"
                        "[Match]\n"
                        "Kind=!*\n"
                        "Type=ether\n"
                        "\n"
                        "[Network]\n"
                        "DHCP=yes\n"
                    ),
                    "destination": "/etc/systemd/network/89-ethernet.network",
                    "permissions": "664",
                },
                {
                    "content": f"{username} ALL=(ALL) NOPASSWD:ALL",
                    "destination": f"/etc/sudoers.d/{username}",
                },
            ],
            "bootloader": {
                "resetType": "hard-reset",
            },
            "kernelCommandLine": {
                "extraCommandLine": [
                    "console=ttyS0,115200",
                    "rd.info",
                    "log_buf_len=1M",
                    "systemd.log_level=debug",
                ],
            },
            "packages": {
                "install": [
                    "openssh-server",
                ],
                # TODO: Remove this remove operation when switching to the seed image. The minimal image is currently
                # being used and it has dracut-hostonly installed, so it needs to be removed before testing, since it
                # prevents the image from booting in a VM and being tested. Removing the package will cause a rebuild of
                # the initramfs to include drivers necessary to find the root filesystem on the disk. While using the
                # minimal image, without removing this package, dracut will hang during the boot process waiting for the
                # root filesystem to be found.
                # Work item: https://dev.azure.com/mariner-org/ECF/_workitems/edit/12153
                "remove": [
                    "dracut-hostonly",
                ],
            },
            "services": {
                "enable": [
                    "sshd",
                ],
            },
            "users": [
                {
                    "name": username,
                    "sshPublicKeys": [
                        ssh_public_key,
                    ],
                },
            ],
        },
        "storage": {
            "bootType": "efi",
            "disks": [
                {
                    "partitionTableType": "gpt",
                    "maxSize": "10G",
                    "partitions": [
                        {
                            "id": "esp",
                            "type": "esp",
                            "size": "8M",
                        },
                        {
                            "id": "rootfs",
                            "type": "root",
                            "size": "grow",
                        },
                    ],
                },
            ],
            "filesystems": [
                {
                    "deviceId": "esp",
                    "type": "fat32",
                    "mountPoint": {
                        "path": "/boot/efi",
                        "options": "umask=0077",
                    },
                },
                {
                    "deviceId": "rootfs",
                    "type": "ext4",
                    "mountPoint": {
                        "path": "/",
                    },
                },
            ],
        },
    }

    # create a repo file for imagecustomizer
    repo_file_path = create_repo_file(test_temp_dir)

    fd, customizer_config_path = tempfile.mkstemp(prefix="imagecustomizer-config-", suffix=".yaml", dir=test_temp_dir)
    with open(fd, mode="w") as file:
        yaml.safe_dump(customizer_config_content, file)

    customizer_config_path_obj = Path(customizer_config_path)
    close_list.append(RemoveFileOnClose(customizer_config_path_obj))

    run_image_customizer_binary(
        image_customizer_binary_path,
        initial_output_image_path,
        customizer_config_path_obj,
        repo_file_path,
        customized_output_image_path,
        output_format,
        customizer_build_dir,
    )

    # Use the customized image for VM testing
    final_image_path = customized_output_image_path
    logging.info(f"Image customization step completed, proceeding to VM creation...")
    logging.info(f"Final image path: {final_image_path}")

    # Verify the customized image exists
    if not final_image_path.exists():
        raise FileNotFoundError(f"Customized image not found at: {final_image_path}")

    image_size = final_image_path.stat().st_size
    logging.info(f"Customized image size: {image_size} bytes ({image_size / (1024 * 1024):.1f} MB)")

    # Step 2: Create VM and test the created image
    logging.info(f"Step 2: Creating VM to test the created image...")

    image_name = os.path.basename(final_image_path)
    image_name_without_ext, image_ext = os.path.splitext(image_name)
    created_image_name = (
        image_name_without_ext
        + "_"
        + get_host_distro()
        + "_"
        + target_boot_type
        + "_created"
        + image_ext
        + "_"
        + target_boot_type
        + "_created"
        + image_ext
    )
    created_image_path = str(logs_dir) + "/" + created_image_name
    vm_console_log_file_path = created_image_path + ".console.log"
    logging.debug(f"- vm_console_log_file_path = {vm_console_log_file_path}")

    vm_image = final_image_path
    if output_format != "iso":
        diff_image_path = test_temp_dir.joinpath("image-diff.qcow2")

        logging.info(f"Creating differencing disk at: {diff_image_path}")

        # Create a differencing disk for the VM. This will make it easier to manually debug
        # Use the output_format as the backing file format
        local_client.run(
            [
                "qemu-img",
                "create",
                "-F",
                output_format,
                "-f",
                "qcow2",
                "-b",
                str(final_image_path),
                str(diff_image_path),
            ],
        ).check_exit_code()

        # Ensure VM can write to the disk file.
        os.chmod(diff_image_path, 0o666)

        vm_image = diff_image_path
        logging.info(f"Using differencing disk for VM: {vm_image}")

    # Create VM.
    vm_name = test_instance_name
    logging.info(f"Creating VM with name: {vm_name}")

    vm_spec = VmSpec(vm_name, 4096, 4, vm_image, target_boot_type, secure_boot)
    logging.info(f"VM spec created with memory: 4096 MB, CPUs: 4, boot type: {target_boot_type}")

    domain_xml = create_libvirt_domain_xml(libvirt_conn, vm_spec)
    logging.info(f"LibVirt domain XML generated")

    logging.debug(f"\n\ndomain_xml            = {domain_xml}\n\n")

    vm = LibvirtVm(vm_name, domain_xml, vm_console_log_file_path, libvirt_conn)
    close_list.append(vm)
    logging.info(f"LibVirt VM object created")

    # Start VM.
    logging.info(f"Starting VM...")
    vm.start()
    logging.info(f"VM started successfully!")

    # Connect to the VM.
    logging.info(f"Attempting to connect to VM via SSH...")
    logging.info(f"Username: {username}")

    with vm.create_ssh_client(ssh_private_key_path, test_temp_dir, username) as ssh_client:
        logging.info(f"SSH connection established successfully!")
        # Run the test
        logging.info(f"Running basic checks on the VM...")
        run_basic_checks(ssh_client, test_temp_dir)
        logging.info(f"Basic checks completed successfully!")


def run_basic_checks(
    ssh_client: SshClient,
    test_temp_dir: Path,
) -> None:

    ssh_client.run("cat /proc/cmdline").check_exit_code()

    os_release_path = test_temp_dir.joinpath("os-release")
    ssh_client.get_file(Path("/etc/os-release"), os_release_path)

    with open(os_release_path, "r") as os_release_fd:
        os_release_text = os_release_fd.read()

        # Since imagecreator creates new images, we expect Azure Linux 3.0
        assert "ID=azurelinux" in os_release_text
        assert 'VERSION_ID="3.0"' in os_release_text

    # Check that essential packages are installed
    ssh_client.run("rpm -q kernel").check_exit_code()
    ssh_client.run("rpm -q systemd").check_exit_code()
    ssh_client.run("rpm -q grub2").check_exit_code()
    ssh_client.run("rpm -q bash").check_exit_code()


@pytest.mark.skipif(platform.machine() != "x86_64", reason="arm64 is not supported for this combination")
def test_create_image_efi_qcow_output(
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
    config_path = IMAGECREATOR_TEST_CONFIGS_DIR.joinpath("minimal-os.yaml")
    output_format = "qcow2"
    # debug message
    logging.debug("Running test_create_image_efi_qcow_output...")

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
    )


def test_create_image_efi_raw_output(
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
    config_path = IMAGECREATOR_TEST_CONFIGS_DIR.joinpath("minimal-os.yaml")
    output_format = "raw"

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
    )
