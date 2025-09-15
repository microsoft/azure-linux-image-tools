# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import logging
from pathlib import Path
from typing import List

from . import local_client


# Run the imagecreator binary tool.
def run_image_creator(
    image_creator_binary_path: Path,
    rpm_sources: List[Path],
    tools_tar: Path,
    config_path: Path,
    output_image_format: str,
    output_image_path: Path,
    build_dir: Path,
) -> None:
    # Build rpm sources arguments
    rpm_source_args = []
    for rpm_source in rpm_sources:
        rpm_source_args.extend(["--rpm-source", str(rpm_source)])

    args = [
        str(image_creator_binary_path),
        "--build-dir",
        str(build_dir),
        "--config-file",
        str(config_path),
        "--tools-file",
        str(tools_tar),
        "--output-image-format",
        output_image_format,
        "--output-image-file",
        str(output_image_path),
        "--log-level",
        "debug",
    ] + rpm_source_args

    logging.info(f"Starting image creation with imagecreator...")
    logging.info(f"Command: {' '.join(args)}")

    # Run the imagecreator binary with sudo and proper environment variables
    result = local_client.run(
        args, timeout=1800, stdout_log_level=logging.INFO, stderr_log_level=logging.INFO  # 30 minutes timeout
    )
    result.check_exit_code()
    logging.info(f"Image creation completed successfully!")


def run_image_customizer_binary(
    image_customizer_binary_path: Path,
    input_image_path: Path,
    config_path: Path,
    output_image_path: Path,
    output_image_format: str,
    build_dir: Path,
    distro: str,
    version: str,
) -> None:
    # Create build directory if it doesn't exist
    build_dir.mkdir(exist_ok=True, parents=True)

    args = [
        str(image_customizer_binary_path),
        "customize",
        "--image-file",
        str(input_image_path),
        "--config-file",
        str(config_path),
        "--build-dir",
        str(build_dir),
        "--output-image-file",
        str(output_image_path),
        "--output-image-format",
        output_image_format,
        "--distro",
        distro,
        "--distro-version",
        version,
        "--log-level",
        "debug",
    ]

    logging.info(f"Starting image customization with imagecustomizer...")
    logging.info(f"Command: {' '.join(args)}")

    # Run the imagecustomizer binary with sudo and proper environment variables
    result = local_client.run(
        args, timeout=1800, stdout_log_level=logging.INFO, stderr_log_level=logging.INFO  # 30 minutes timeout
    )
    result.check_exit_code()
    logging.info(f"Image customization completed successfully!")
