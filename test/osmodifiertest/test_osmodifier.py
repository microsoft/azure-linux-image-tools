# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import logging
import os
import platform
import tempfile
from getpass import getuser
from pathlib import Path
from typing import Generator, List, Tuple

import libvirt  # type: ignore
import pytest
import yaml
from docker import DockerClient
from vmtests.vmtests.utils import local_client
from vmtests.vmtests.utils.closeable import Closeable
from vmtests.vmtests.utils.host_utils import get_host_distro
from vmtests.vmtests.utils.imagecustomizer import run_image_customizer
from vmtests.vmtests.utils.libvirt_utils import VmSpec, create_libvirt_domain_xml
from vmtests.vmtests.utils.libvirt_vm import LibvirtVm
from vmtests.vmtests.utils.ssh_client import SshClient

from .conftest import TEST_CONFIGS_DIR


@pytest.fixture(scope="session")
def setup_vm_with_osmodifier(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    osmodifier_binary: Path,
    input_image: Path,
    ssh_key: Tuple[str, Path],
    session_temp_dir: Path,
    test_instance_name: str,
    logs_dir: Path,
    libvirt_conn: libvirt.virConnect,
    close_list: List[Closeable],
) -> Generator[Tuple[SshClient, Path, Path], None, None]:
    if platform.machine() == "x86_64":
        config_path = TEST_CONFIGS_DIR.joinpath("os-vm-config.yaml")
    else:
        config_path = TEST_CONFIGS_DIR.joinpath("os-vm-config-arm64.yaml")

    output_format = "qcow2"
    (ssh_public_key, ssh_private_key_path) = ssh_key
    secure_boot = False

    source_boot_type = "efi"
    if Path(input_image).suffix.lower() == ".vhd":
        source_boot_type = "legacy"

    target_boot_type = source_boot_type
    if output_format == "iso":
        target_boot_type = "efi"

    output_image_path = session_temp_dir.joinpath("image." + output_format)
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

    image_name = os.path.basename(output_image_path)
    image_name_without_ext, image_ext = os.path.splitext(image_name)
    customized_image_name = (
        image_name_without_ext
        + "_"
        + get_host_distro()
        + "_"
        + source_boot_type
        + "_to_"
        + target_boot_type
        + image_ext
    )
    customized_image_path = str(logs_dir) + "/" + customized_image_name
    vm_console_log_file_path = customized_image_path + ".console.log"
    logging.debug(f"- vm_console_log_file_path = {vm_console_log_file_path}")

    vm_image = output_image_path
    diff_image_path = session_temp_dir.joinpath("image-diff.qcow2")

    # Create a differencing disk for the VM.
    # This will make it easier to manually debug what is in the image itself and what was set during first boot.
    local_client.run(
        [
            "qemu-img",
            "create",
            "-F",
            "qcow2",
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
    domain_xml = create_libvirt_domain_xml(
        libvirt_conn, vm_spec, vm_console_log_file_path
    )

    logging.debug(f"\n\ndomain_xml            = {domain_xml}\n\n")

    vm = LibvirtVm(vm_name, domain_xml, libvirt_conn)
    close_list.append(vm)

    # Start VM.
    vm.start()

    remote_osmodifier_path = Path("/tmp/osmodifier")

    ssh_client = vm.create_ssh_client(ssh_private_key_path, session_temp_dir)
    ssh_client.put_file(osmodifier_binary, remote_osmodifier_path)
    ssh_client.run(f"sudo chmod +x {remote_osmodifier_path}").check_exit_code()

    return ssh_client, remote_osmodifier_path, logs_dir


def run_osmodifier_with_config(
    setup_vm_with_osmodifier: Tuple[SshClient, Path, Path],
    config_filename: str,
    log_path: Path,
) -> None:
    ssh_client, remote_osmodifier_path, _ = setup_vm_with_osmodifier

    # Flatten the config (removing top-level 'os') and write to temp file
    local_config = TEST_CONFIGS_DIR / config_filename
    with open(local_config, "r") as f:
        content = yaml.safe_load(f)

    flattened = content.get("os", content)
    with tempfile.NamedTemporaryFile("w", suffix=".yaml", delete=False) as tmp:
        yaml.safe_dump(flattened, tmp, sort_keys=False)
        tmp_config_path = Path(tmp.name)

    # Upload config
    remote_config_path = Path("/tmp") / config_filename
    ssh_client.put_file(tmp_config_path, remote_config_path)

    # Run osmodifier
    result = ssh_client.run(
        f"sudo {remote_osmodifier_path} --config-file {remote_config_path}",
        stdout_log_level=logging.INFO,
        stderr_log_level=logging.INFO,
    )

    log_path.parent.mkdir(parents=True, exist_ok=True)
    with open(log_path, "w") as f:
        f.write(result.stdout + result.stderr)

    result.check_exit_code()


def check_services(ssh_client, service: str, expected: str) -> None:
    cmd = f"systemctl is-enabled {service} || true"
    output = ssh_client.run(cmd).stdout.strip().splitlines()
    if expected == "enabled":
        assert "enabled" in output, f"{service} expected 'enabled', got {output}"
    elif expected == "disabled":
        assert all(
            line != "enabled" for line in output
        ), f"{service} expected 'disabled', got {output}"


def test_modify_services(
    setup_vm_with_osmodifier: Tuple[SshClient, Path, Path],
) -> None:
    """
    Tests that osmodifier enables and disables specific services correctly.
    """
    ssh_client, _, logs_dir = setup_vm_with_osmodifier

    run_osmodifier_with_config(
        setup_vm_with_osmodifier,
        "services-config.yaml",
        logs_dir / "test_services.log",
    )

    check_services(ssh_client, "console-getty", "enabled")
    check_services(ssh_client, "chronyd", "disabled")


def test_modify_kernel_modules(
    setup_vm_with_osmodifier: Tuple[SshClient, Path, Path],
) -> None:
    """
    Verifies osmodifier correctly configures kernel module loading and options.
    """
    ssh_client, _, logs_dir = setup_vm_with_osmodifier

    # Run osmodifier with kernel modules config
    run_osmodifier_with_config(
        setup_vm_with_osmodifier,
        "modules-config.yaml",
        logs_dir / "test_modules.log",
    )

    module_disabled_path = "/etc/modprobe.d/modules-disabled.conf"
    module_load_path = "/etc/modules-load.d/modules-load.conf"
    module_options_path = "/etc/modprobe.d/module-options.conf"

    # Read contents from the VM
    load_content = ssh_client.run(f"cat {module_load_path} || true").stdout
    assert "vfio" in load_content
    assert "mlx5_ib" in load_content

    disabled_content = ssh_client.run(f"cat {module_disabled_path} || true").stdout
    assert "blacklist mousedev" in disabled_content

    options_content = ssh_client.run(f"cat {module_options_path} || true").stdout
    assert "options vfio" in options_content
    assert "enable_unsafe_noiommu_mode=Y" in options_content
    assert "disable_vga=Y" in options_content
    assert "options e1000e InterruptThrottleRate=3000,3000,3000" in options_content


def test_update_hostname(
    setup_vm_with_osmodifier: Tuple[SshClient, Path, Path],
) -> None:
    """
    Test that osmodifier correctly updates the system hostname inside the VM.
    """
    ssh_client, _, logs_dir = setup_vm_with_osmodifier

    run_osmodifier_with_config(
        setup_vm_with_osmodifier,
        "hostname-config.yaml",
        logs_dir / "test_hostname.log",
    )

    # Verify hostname
    actual_hostname = ssh_client.run("cat /etc/hostname").stdout.strip()
    expected_hostname = "testname"
    assert (
        actual_hostname == expected_hostname
    ), f"Expected hostname '{expected_hostname}', got '{actual_hostname}'"
