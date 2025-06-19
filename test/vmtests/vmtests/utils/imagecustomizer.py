# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import logging
import tempfile
from os import fdopen
from pathlib import Path
from typing import Any, Dict, List

import yaml
from docker import DockerClient

from .closeable import Closeable
from .docker_utils import container_run
from .file_utils import RemoveFileOnClose

# App Insights Staging Connection String.
AZURE_CONN_STR = "InstrumentationKey=e0c67213-5e25-4ef2-8f93-c283e8b93629;IngestionEndpoint=https://eastus2-3.in.applicationinsights.azure.com/;ApplicationId=f215fd6d-af24-4bd3-acfa-212cb0c916dc"


# Run the containerized version of the imagecustomizer tool.
def run_image_customizer(
    docker_client: DockerClient,
    image_customizer_container_url: str,
    base_image_path: Path,
    config_path: Path,
    ssh_username: str,
    ssh_public_key: str,
    output_image_format: str,
    output_image_path: Path,
    close_list: List[Closeable],
) -> None:
    modified_config_path = None

    container_base_image_dir = Path("/container/base_image")
    container_config_dir = Path("/container/config")
    container_output_image_dir = Path("/container/output_image")
    container_build_dir = Path("/container/build")

    base_image_dir = base_image_path.parent.absolute()
    config_dir = config_path.parent.absolute()
    output_image_dir = output_image_path.parent.absolute()

    # Add SSH server and SSH public key to config file.
    modified_config_path = add_ssh_to_config(config_path, ssh_username, ssh_public_key, close_list)

    container_base_image_path = container_base_image_dir.joinpath(base_image_path.name)
    container_config_path = container_config_dir.joinpath(modified_config_path.name)
    container_output_image_path = container_output_image_dir.joinpath(output_image_path.name)

    args = [
        "imagecustomizer",
        "--image-file",
        str(container_base_image_path),
        "--config-file",
        str(container_config_path),
        "--build-dir",
        str(container_build_dir),
        "--output-image-format",
        output_image_format,
        "--output-image-file",
        str(container_output_image_path),
        "--log-level",
        "debug",
    ]

    volumes = [
        f"{base_image_dir}:{container_base_image_dir}:z",
        f"{config_dir}:{container_config_dir}:z",
        f"{output_image_dir}:{container_output_image_dir}:z",
        "/dev:/dev",
    ]

    environment = {
        "ENABLE_TELEMETRY": "true",
        "AZURE_MONITOR_CONNECTION_STRING": AZURE_CONN_STR,
    }

    entrypoint = "/usr/local/bin/entrypoint.sh"

    container_run(
        docker_client,
        image_customizer_container_url,
        args,
        detach=True,
        privileged=True,
        volumes=volumes,
        environment=environment,
        entrypoint=entrypoint,
    )


# Modify an image customizer config file:
# - Install the SSH server package,
# - Add a user with an SSH public key.
def add_ssh_to_config(config_path: Path, username: str, ssh_public_key: str, close_list: List[Closeable]) -> Path:
    config_str = config_path.read_text()
    config = yaml.safe_load(config_str)

    logging.debug(str(config))

    os = dict_get_or_set(config, "os", {})

    # Add SSH package.
    packages = dict_get_or_set(os, "packages", {})
    packages_install = dict_get_or_set(packages, "install", [])
    packages_install.append("openssh-server")

    # Enable SSH service.
    services = dict_get_or_set(os, "services", {})
    services_enable = dict_get_or_set(services, "enable", [])
    services_enable.append("sshd")

    # Add user to config.
    user = {
        "name": username,
        "sshPublicKeys": [
            ssh_public_key,
        ],
    }

    users = dict_get_or_set(os, "users", [])
    users.append(user)

    # Allow sudo to be used without password.
    sudoers_add_file = {
        "content": f"{username} ALL=(ALL) NOPASSWD:ALL",
        "destination": f"/etc/sudoers.d/{username}",
    }

    additional_files = dict_get_or_set(os, "additionalFiles", [])
    additional_files.append(sudoers_add_file)

    # Write out new config file to a temporary file.
    fd, modified_config_path = tempfile.mkstemp(prefix=config_path.name + "~", suffix=".tmp", dir=config_path.parent)
    with fdopen(fd, mode="w") as file:
        yaml.safe_dump(config, file)

    path = Path(modified_config_path)
    close_list.append(RemoveFileOnClose(path))
    return path


def dict_get_or_set(dictionary: Dict[Any, Any], value_name: str, default: Any = None) -> Any:
    value = dictionary.get(value_name)
    if value is None:
        dictionary[value_name] = default
    return dictionary[value_name]
