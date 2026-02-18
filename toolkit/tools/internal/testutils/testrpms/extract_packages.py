#!/usr/bin/env python3
"""
Extract package list from a YAML configuration file.

Reads packages from 'os.packages.install' and resolves any 'os.packages.installLists' entries.
"""

from typing import Any
import yaml
import argparse
import os


def extract_packages_from_list_file(file_path: str) -> list[str]:
    """Extract packages from a package list YAML file (top-level 'packages' key)."""
    with open(file_path, 'r') as f:
        data: dict[str, Any] = yaml.safe_load(f)

    return data.get('packages', [])


def extract_packages(config_file: str) -> list[str]:
    """Extract packages from a config file.

    Reads 'os.packages.install' and resolves any 'os.packages.installLists' entries
    (paths to package list files, relative to the config file directory).
    """
    with open(config_file, 'r') as f:
        data: dict[str, dict[str, Any]] = yaml.safe_load(f)

    all_packages: list[str] = []

    os_section: dict[str, Any] = data.get('os', {})
    packages_section: dict[str, Any] = os_section.get('packages', {})
    install_packages: list[str] = packages_section.get('install', [])
    all_packages.extend(install_packages)

    install_lists: list[str] = packages_section.get('installLists', [])
    config_dir = os.path.dirname(config_file)

    for list_file in install_lists:
        list_file_path = os.path.join(config_dir, list_file)
        list_packages = extract_packages_from_list_file(list_file_path)
        all_packages.extend(list_packages)

    return all_packages


def main() -> None:
    parser = argparse.ArgumentParser(description='Extract package list from a YAML configuration file.')
    parser.add_argument(
        'config_file',
        help='Path to the YAML configuration file',
    )
    parser.add_argument(
        '--output-format',
        choices=['space-separated', 'newline-separated'],
        default='space-separated',
        help='Output format for packages',
    )

    args = parser.parse_args()

    packages = extract_packages(args.config_file)

    if args.output_format == 'space-separated':
        print(' '.join(packages))
    else:
        for package in packages:
            print(package)


if __name__ == '__main__':
    main()
