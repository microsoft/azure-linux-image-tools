#!/usr/bin/env python3
"""
Extract package list from YAML configuration files.
"""

import yaml
import sys
import argparse

def extract_packages(config_file_path):
    """Extract packages from a YAML configuration file."""
    try:
        with open(config_file_path, 'r') as f:
            data = yaml.safe_load(f)
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
