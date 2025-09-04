#!/usr/bin/env python3
"""
Extract package list from YAML configuration files.
"""

import yaml
import sys
import argparse
import os

def extract_packages_from_file(config_file_path):
    """Extract packages from a single YAML file."""
    try:
        with open(config_file_path, 'r') as f:
            data = yaml.safe_load(f)
            
            # Handle direct package list (like in golang.yaml)
            if 'packages' in data and data['packages']:
                return data['packages']
            
            packages = data.get('os', {}).get('packages', {}).get('install', [])
            return packages
            
    except FileNotFoundError:
        print(f"Error: Configuration file '{config_file_path}' not found", file=sys.stderr)
        return []
    except yaml.YAMLError as e:
        print(f"Error parsing YAML file '{config_file_path}': {e}", file=sys.stderr)
        return []
    except Exception as e:
        print(f"Error reading file '{config_file_path}': {e}", file=sys.stderr)
        return []

def extract_packages(config_file_path):
    """Extract packages from a YAML configuration file, including installLists."""
    all_packages = []
    
    try:
        with open(config_file_path, 'r') as f:
            data = yaml.safe_load(f)
            
            # Get packages from os.packages.install
            packages = data.get('os', {}).get('packages', {})
            install_packages = packages.get('install', [])
            all_packages.extend(install_packages)
            
            # Get packages from os.packages.installLists
            install_lists = packages.get('installLists', [])
            config_dir = os.path.dirname(config_file_path)
            
            for list_file in install_lists:
                # Resolve relative path relative to the config file directory
                list_file_path = os.path.join(config_dir, list_file)
                list_packages = extract_packages_from_file(list_file_path)
                all_packages.extend(list_packages)
            
            return all_packages
            
    except FileNotFoundError:
        print(f"Error: Configuration file '{config_file_path}' not found", file=sys.stderr)
        return []
    except yaml.YAMLError as e:
        print(f"Error parsing YAML file '{config_file_path}': {e}", file=sys.stderr)
        return []
    except Exception as e:
        print(f"Error reading file '{config_file_path}': {e}", file=sys.stderr)
        return []

def main():
    parser = argparse.ArgumentParser(description='Extract package list from YAML configuration file')
    parser.add_argument('config_file', help='Path to the YAML configuration file')
    parser.add_argument('--output-format', choices=['space-separated', 'newline-separated'], 
                       default='space-separated', help='Output format for packages')
    
    args = parser.parse_args()
    
    packages = extract_packages(args.config_file)
    
    if args.output_format == 'space-separated':
        print(' '.join(packages))
    else:
        for package in packages:
            print(package)

if __name__ == '__main__':
    main()