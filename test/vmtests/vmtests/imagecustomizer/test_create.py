# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import logging
import os
import platform
from pathlib import Path
from typing import Any, Dict, List, Tuple

import libvirt  # type: ignore
from docker import DockerClient

from ..conftest import TEST_CONFIGS_DIR
from ..utils import local_client
from ..utils.closeable import Closeable
from ..utils.host_utils import get_host_distro
from ..utils.imagecustomizer import add_preview_features_to_config, add_ssh_to_config, run_image_customizer
from ..utils.libvirt_utils import VmSpec, create_libvirt_domain_xml
from ..utils.libvirt_vm import LibvirtVm
from ..utils.ssh_client import SshClient
from ..utils.user_utils import get_username

# Common packages that should be present in all distributions
COMMON_PACKAGES = ["kernel", "systemd", "bash"]

# Architecture-specific GRUB EFI package for Fedora
FEDORA_GRUB_PKG = "grub2-efi-x64" if platform.machine() == "x86_64" else "grub2-efi-aa64"
AZURELINUX_GRUB_PKG = "grub2"

# Distribution-specific configuration
DISTRO_CONFIGS: Dict[str, Dict[str, Any]] = {
    "fedora": {
        "os_release": {
            "ID": "fedora",
            "VERSION_ID": "42",
        },
        "packages": COMMON_PACKAGES + [FEDORA_GRUB_PKG],
    },
    "azurelinux": {
        "os_release": {
            "ID": "azurelinux",
            "VERSION_ID": "3.0",
        },
        "packages": COMMON_PACKAGES + [AZURELINUX_GRUB_PKG],
    },
}


def verify_packages(ssh_client: SshClient, packages_to_check: List[str]) -> None:
    """Verify that specified packages are present in the system.

    Args:
        ssh_client: SSH client connected to the target system
        packages_to_check: List of package names to verify
    """
    for package in packages_to_check:
        ssh_client.run(f"rpm -q {package}").check_exit_code()


def verify_os_release(os_release_text: str, expected_values: Dict[str, str]) -> None:
    """Verify os-release content matches expected values.

    Args:
        os_release_text: Content of os-release file
        expected_values: Dictionary of key-value pairs to verify
    """

    # Parse os-release into a dictionary
    os_release_dict = {}
    for line in os_release_text.splitlines():
        if "=" in line:
            key, value = line.split("=", 1)
            # Remove quotes if present
            value = value.strip('"')
            os_release_dict[key] = value

    for key, value in expected_values.items():
        if key not in os_release_dict:
            raise AssertionError(f"Key '{key}' not found in os-release")
        if os_release_dict[key] != value:
            raise AssertionError(f"Value mismatch for '{key}': expected '{value}', got '{os_release_dict[key]}'")


def run_basic_checks(
    ssh_client: SshClient,
    test_temp_dir: Path,
    distro: str,
) -> None:
    """Run basic checks for the specified distribution.

    Args:
        ssh_client: SSH client for running commands
        test_temp_dir: Temporary directory for test artifacts
        distro: Distribution name (must be a key in DISTRO_CONFIGS)
    """
    if distro not in DISTRO_CONFIGS:
        raise ValueError(f"Unsupported distribution: {distro}")

    config = DISTRO_CONFIGS[distro]

    # Check kernel cmdline
    ssh_client.run("cat /proc/cmdline").check_exit_code()

    # Get and verify os-release
    os_release_path = test_temp_dir.joinpath("os-release")
    ssh_client.get_file(Path("/etc/os-release"), os_release_path)

    with open(os_release_path, "r") as os_release_fd:
        os_release_text = os_release_fd.read()
        verify_os_release(os_release_text, config["os_release"])

    # Check required packages
    verify_packages(ssh_client, config["packages"])


def run_create_image_test(
    image_customizer_container_url: str,
    docker_client: DockerClient,
    rpm_sources: List[Path],
    tools_file: Path,
    config_path: Path,
    output_format: str,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
    logs_dir: Path,
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
    distro: str,
    version: str,
) -> None:

    (ssh_public_key, ssh_private_key_path) = ssh_key

    secure_boot = False
    target_boot_type = "efi"

    # Step 1: Create initial image with imagecustomizer create subcommand
    initial_output_image_path = test_temp_dir.joinpath("initial-image." + output_format)

    logging.info("Step 1: Creating initial image with imagecustomizer create subcommand")
    logging.debug("Test parameters:")
    logging.debug(f"- image_customizer_container_url = {image_customizer_container_url}")
    logging.debug(f"- rpm_sources                    = {rpm_sources}")
    logging.debug(f"- tools_file                     = {tools_file}")
    logging.debug(f"- config_path                    = {config_path}")
    logging.debug(f"- output_format                  = {output_format}")
    logging.debug(f"- target_boot_type               = {target_boot_type}")
    logging.debug(f"- logs_dir                       = {logs_dir}")

    username = get_username()

    final_config_path = config_path
    if distro.lower() == "fedora":
        final_config_path = add_preview_features_to_config(config_path, "fedora-42", close_list)

    run_image_customizer(
        docker_client,
        image_customizer_container_url,
        "create",
        final_config_path,
        output_format,
        initial_output_image_path,
        rpm_sources=rpm_sources,
        tools_file=tools_file,
        distro=distro,
        distro_version=version,
    )

    # Step 1.5: Run imagecustomizer to add SSH configuration
    logging.info(f"Step 1.5: Running imagecustomizer to add SSH configuration")

    customized_output_image_path = test_temp_dir.joinpath("customized-image." + output_format)

    # Use base SSH config and add SSH user configuration dynamically
    base_ssh_config_path = TEST_CONFIGS_DIR.joinpath("ssh-base-config.yaml")
    customizer_config_path_obj = add_ssh_to_config(base_ssh_config_path, username, ssh_public_key, close_list)

    # Add Fedora preview features if needed
    if distro.lower() == "fedora":
        customizer_config_path_obj = add_preview_features_to_config(customizer_config_path_obj, "fedora-42", close_list)

    run_image_customizer(
        docker_client,
        image_customizer_container_url,
        "customize",
        customizer_config_path_obj,
        output_format,
        customized_output_image_path,
        image_file=initial_output_image_path,
    )

    # Use the customized image for VM testing
    final_image_path = customized_output_image_path
    logging.info(f"Image customization step completed, proceeding to VM creation...")
    logging.info(f"Final image path: {final_image_path}")

    # Verify the customized image exists
    if not final_image_path.exists():
        raise FileNotFoundError(f"Customized image not found at: {final_image_path}")

    image_size = final_image_path.stat().st_size
    logging.info(f"Customized image size: {image_size} bytes ({image_size / (1024 * 1024):.1f} MiB)")

    # Step 2: Create VM and test the created image
    logging.info(f"Step 2: Creating VM to test the created image")

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
    logging.info(f"Starting VM")
    vm.start()
    logging.info(f"VM started successfully!")

    # Connect to the VM.
    logging.info(f"Attempting to connect to VM via SSH")
    logging.info(f"Username: {username}")

    with vm.create_ssh_client(ssh_private_key_path, test_temp_dir, username) as ssh_client:
        logging.info(f"SSH connection established successfully!")
        # Run the test
        logging.info(f"Running basic checks on the VM")
        run_basic_checks(ssh_client, test_temp_dir, distro)
        logging.info(f"Basic checks completed successfully!")


def test_create_image_efi_qcow_output_azl3(
    image_customizer_container_url: str,
    docker_client: DockerClient,
    rpm_sources_azl3: Path,
    tools_file_azl3: Path,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
    logs_dir: Path,
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
) -> None:
    run_create_image_test(
        image_customizer_container_url,
        docker_client,
        [rpm_sources_azl3],
        tools_file_azl3,
        TEST_CONFIGS_DIR.joinpath("create-minimal-os.yaml"),
        "qcow2",
        ssh_key,
        test_temp_dir,
        test_instance_name,
        logs_dir,
        libvirt_conn,
        close_list,
        "azurelinux",
        "3.0",
    )


def test_create_image_efi_qcow_output_fedora(
    image_customizer_container_url: str,
    docker_client: DockerClient,
    rpm_sources_fedora42: Path,
    tools_file_fedora42: Path,
    ssh_key: Tuple[str, Path],
    test_temp_dir: Path,
    test_instance_name: str,
    logs_dir: Path,
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
) -> None:
    if platform.machine() == "x86_64":
        config_path = TEST_CONFIGS_DIR.joinpath("create-fedora-amd64.yaml")
    else:
        config_path = TEST_CONFIGS_DIR.joinpath("create-fedora-arm64.yaml")

    run_create_image_test(
        image_customizer_container_url,
        docker_client,
        [rpm_sources_fedora42],
        tools_file_fedora42,
        config_path,
        "qcow2",
        ssh_key,
        test_temp_dir,
        test_instance_name,
        logs_dir,
        libvirt_conn,
        close_list,
        "fedora",
        "42",
    )
